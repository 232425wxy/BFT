package gossip

import (
	protogossip "BFT/proto/gossip"
	"bytes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gtank/merlin"
	pool "github.com/libp2p/go-buffer-pool"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/box"

	"BFT/crypto"
	"BFT/crypto/ed25519"
	cryptoenc "BFT/crypto/encoding"
	"BFT/libs/async"
	"BFT/libs/protoio"
)

// 4 + 1024 == 1028 total frame size
const (
	dataLenSize = 4    // 用来表示数据长度的字段，该字段大小为 4 字节
	dataMaxSize = 1024 // 数据最大大小为 1024 字节
	// 数据帧大小，数据帧由两部分组成，第一部分表示数据大小字段，第二部分表示数据字段，所以数据帧大小为 1028 字节
	totalFrameSize   = dataLenSize + dataMaxSize
	aeadSizeOverhead = 16                         // 认证标签的开销，为 16 字节
	aeadKeySize      = chacha20poly1305.KeySize   // 秘钥大小，长度为 32 字节
	aeadNonceSize    = chacha20poly1305.NonceSize // nonce 的大小，为 12 字节
)

var (
	labelEphemeralLowerPublicKey = []byte("EPHEMERAL_LOWER_PUBLIC_KEY")
	labelEphemeralUpperPublicKey = []byte("EPHEMERAL_HIGHER_PUBLIC_KEY")
	labelDHSecret                = []byte("DH_SECRET")
	labelSecretConnMac           = []byte("SECRET_CONNECTION_MAC")

	secretConnKeyAndChallengeGen = []byte("SRBFT_SECRET_CONNECTION_KEY_AND_CHALLENGE_GEN")
)

// SecretConn 实现了net.Conn，它是STS协议的一个实现。
//
// SecretConn 负责根据已知信息验证远端 peer 的 pubkey，比如 nodeID。
// 否则，它们很容易受到中间人的攻击。
type SecretConn struct {

	// 不可变的
	// recvAead 用来解密收到的数据
	recvAead cipher.AEAD
	// sendAead 用来加密发送的数据
	sendAead cipher.AEAD
	// peer 节点的公钥
	remotePubKey crypto.PubKey
	conn         io.ReadWriteCloser

	recvMutex  sync.Mutex
	recvBuffer []byte
	recvNonce  *[aeadNonceSize]byte

	sendMutex sync.Mutex
	sendNonce *[aeadNonceSize]byte
}

// MakeSecretConn 建立加密连接，需要经过以下几个步骤：
//		1. 通过私钥获取公钥
//		2. 获得本地的临时公私钥对
//		3. 与对方互相交换临时公钥
//		4. 对本地临时公钥和对方的临时公钥按大小比较一下
//		5. 根据对方的临时公钥和本地的临时私钥，计算一个 DH 秘密值
//		6. 根据第 4 步和第 5 步计算出来的值，算出接收秘密值和发送秘密值
//		7. 生成挑战值
//		8. 根据接收秘密值和发送秘密值计算出接收秘钥和发送秘钥
//		9. 利用本地私钥给挑战值进行签名，得到签名消息
//		10. 与对方互换本地公钥与签名消息
//		11. 验证对方的公钥与签名消息
//		12. 验证通过后，保存对方的公钥
func MakeSecretConn(conn io.ReadWriteCloser, locPrivKey crypto.PrivKey) (*SecretConn, error) {
	// 1. 通过私钥获取公钥
	var (
		locPubKey = locPrivKey.PubKey()
	)

	// 2. 获得本地的临时公私钥对
	locEphPub, locEphPriv := genEphKeys()

	// 3. 与对方互相交换临时公钥
	remEphPub, err := shareEphPubKey(conn, locEphPub)
	if err != nil {
		return nil, err
	}

	// 4. 将本地临时公钥和对方的临时公钥按大小比较一下
	loEphPub, hiEphPub := sort32(locEphPub, remEphPub)

	// merlin 是用来实现零知识证明的
	transcript := merlin.NewTranscript("SRBFT_SECRET_CONNECTION_TRANSCRIPT_HASH")

	transcript.AppendMessage(labelEphemeralLowerPublicKey, loEphPub[:])
	transcript.AppendMessage(labelEphemeralUpperPublicKey, hiEphPub[:])

	// 检查本地临时公钥是否比 peer 的临时公钥小
	locIsLeast := bytes.Equal(locEphPub[:], loEphPub[:])

	// 5. 根据对方的临时公钥和本地的临时私钥，计算一个 DH 秘密值
	dhSecret, err := computeDHSecret(remEphPub, locEphPriv)
	if err != nil {
		return nil, err
	}

	transcript.AppendMessage(labelDHSecret, dhSecret[:])

	// 6. 根据第 4 步和第 5 步计算出来的值，算出接收秘密值和发送秘密值
	recvSecret, sendSecret := deriveSecrets(dhSecret, locIsLeast)
	// 7. 生成挑战值
	const challengeSize = 32
	var challenge [challengeSize]byte
	challengeSlice := transcript.ExtractBytes(labelSecretConnMac, challengeSize)

	copy(challenge[:], challengeSlice[0:challengeSize])
	// 8. 根据接收秘密值和发送秘密值计算出接收秘钥和发送秘钥
	sendAead, err := chacha20poly1305.New(sendSecret[:])
	if err != nil {
		return nil, errors.New("invalid send SecretConn Key")
	}
	recvAead, err := chacha20poly1305.New(recvSecret[:])
	if err != nil {
		return nil, errors.New("invalid receive SecretConn Key")
	}

	sc := &SecretConn{
		conn:       conn,
		recvBuffer: nil,
		recvNonce:  new([aeadNonceSize]byte),
		sendNonce:  new([aeadNonceSize]byte),
		recvAead:   recvAead,
		sendAead:   sendAead,
	}

	// 9. 利用本地私钥给挑战值进行签名，得到签名消息
	locSignature, err := signChallenge(&challenge, locPrivKey)
	if err != nil {
		return nil, err
	}

	// 10. 与对方互换本地公钥与签名消息
	authSigMsg, err := shareAuthSignature(sc, locPubKey, locSignature)
	if err != nil {
		return nil, err
	}

	remPubKey, remSignature := authSigMsg.Key, authSigMsg.Sig
	if _, ok := remPubKey.(ed25519.PubKey); !ok {
		return nil, fmt.Errorf("expected ed25519 pubkey, got %T", remPubKey)
	}

	// 11. 验证对方的公钥与签名消息
	if !remPubKey.VerifySignature(challenge[:], remSignature) {
		return nil, errors.New("challenge verification failed")
	}

	// 12. 验证通过后，保存对方的公钥
	sc.remotePubKey = remPubKey

	return sc, nil
}

// RemotePubKey returns authenticated remote pubkey
func (sc *SecretConn) RemotePubKey() crypto.PubKey {
	return sc.remotePubKey
}

// Write 将数据封装成帧，然后加密以后发送出去
func (sc *SecretConn) Write(data []byte) (n int, err error) {
	sc.sendMutex.Lock()
	defer sc.sendMutex.Unlock()

	for 0 < len(data) {
		if err := func() error {
			// 从缓冲池里拿出 16 + 1028 字节的空间，作为封装数据帧
			// 封装数据帧 = 认证标签(16字节) + 数据长度字段(4字节) + 数据(1024字节)
			var sealedFrame = pool.Get(aeadSizeOverhead + totalFrameSize)
			var frame = pool.Get(totalFrameSize)
			defer func() {
				pool.Put(sealedFrame)
				pool.Put(frame)
			}()
			var chunk []byte
			if dataMaxSize < len(data) {
				// 如果要发送的数据长度大于最大允许长度，则需要分割成更小的块发送出去
				chunk = data[:dataMaxSize]
				data = data[dataMaxSize:]
			} else {
				chunk = data
				data = nil
			}
			chunkLength := len(chunk)
			// 表示数据大小的信息放到数据帧的前四个字节中
			binary.LittleEndian.PutUint32(frame, uint32(chunkLength))
			copy(frame[dataLenSize:], chunk)

			// 给数据帧加密
			sc.sendAead.Seal(sealedFrame[:0], sc.sendNonce[:], frame, nil)
			incrNonce(sc.sendNonce)
			// 结束加密

			_, err = sc.conn.Write(sealedFrame)
			if err != nil {
				return err
			}
			n += len(chunk)
			return nil
		}(); err != nil {
			return n, err
		}
	}
	return n, err
}

// CONTRACT: data smaller than dataMaxSize is read atomically.
func (sc *SecretConn) Read(data []byte) (n int, err error) {
	sc.recvMutex.Lock()
	defer sc.recvMutex.Unlock()

	// 如果不为空，则读取并更新 recvBuffer
	if 0 < len(sc.recvBuffer) {
		n = copy(data, sc.recvBuffer)
		sc.recvBuffer = sc.recvBuffer[n:]
		return
	}

	// 从底层 net.Conn 里读取数据
	var sealedFrame = pool.Get(aeadSizeOverhead + totalFrameSize)
	defer pool.Put(sealedFrame)
	// 解密加密过的数据帧
	_, err = io.ReadFull(sc.conn, sealedFrame)
	if err != nil {
		return
	}

	// 解密加密过的数据帧
	var frame = pool.Get(totalFrameSize)
	defer pool.Put(frame)
	_, err = sc.recvAead.Open(frame[:0], sc.recvNonce[:], sealedFrame, nil)
	if err != nil {
		return n, fmt.Errorf("failed to decrypt SecretConn: %w", err)
	}
	incrNonce(sc.recvNonce)
	// 结束解密

	// 从解密后的数据帧里读取前四个字节，这前四个字节表示后面跟的数据大小，单位为字节
	var chunkLength = binary.LittleEndian.Uint32(frame) // read the first four bytes
	if chunkLength > dataMaxSize {
		return 0, errors.New("chunkLength is greater than dataMaxSize")
	}
	// 将数据帧里的数据部分提取出来，并保存到 data 中
	var chunk = frame[dataLenSize : dataLenSize+chunkLength]
	n = copy(data, chunk)
	if n < len(chunk) {
		sc.recvBuffer = make([]byte, len(chunk)-n)
		copy(sc.recvBuffer, chunk[n:])
	}

	return n, err
}

// Implements net.Conn
func (sc *SecretConn) Close() error                  { return sc.conn.Close() }
func (sc *SecretConn) LocalAddr() net.Addr           { return sc.conn.(net.Conn).LocalAddr() }
func (sc *SecretConn) RemoteAddr() net.Addr          { return sc.conn.(net.Conn).RemoteAddr() }
func (sc *SecretConn) SetDeadline(t time.Time) error { return sc.conn.(net.Conn).SetDeadline(t) }
func (sc *SecretConn) SetReadDeadline(t time.Time) error {
	return sc.conn.(net.Conn).SetReadDeadline(t)
}
func (sc *SecretConn) SetWriteDeadline(t time.Time) error {
	return sc.conn.(net.Conn).SetWriteDeadline(t)
}

// genEphKeys 生成一对临时公钥/私钥对，适合与Seal和Open一起使用。
func genEphKeys() (ephPub, ephPriv *[32]byte) {
	var err error
	ephPub, ephPriv, err = box.GenerateKey(crand.Reader)
	if err != nil {
		panic("Could not generate ephemeral key-pair")
	}
	return
}

// shareEphPubKey 与对等节点交换临时公钥，返回的结果是对方的临时公钥和程序执行时可能出现的错误
func shareEphPubKey(conn io.ReadWriter, locEphPub *[32]byte) (remEphPub *[32]byte, err error) {

	// 发送我们的临时公钥给对方，同时接收它们的临时公钥
	var trs, _ = async.Parallel(
		func(_ int) (val interface{}, abort bool, err error) {
			lc := *locEphPub
			// 将我们自己的本地临时公钥通过 protobuf 序列化后发送给对方
			_, err = protoio.NewDelimitedWriter(conn).WriteMsg(&gogotypes.BytesValue{Value: lc[:]})
			if err != nil {
				return nil, true, err // abort
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			var bytes gogotypes.BytesValue
			// 接收对方的临时公钥
			_, err = protoio.NewDelimitedReader(conn, 1024*1024).ReadMsg(&bytes)
			if err != nil {
				return nil, true, err // abort
			}

			var _remEphPub [32]byte
			copy(_remEphPub[:], bytes.Value)
			return _remEphPub, false, nil
		},
	)

	// 如果出现了错误就返回 nil 和错误
	if trs.FirstError() != nil {
		err = trs.FirstError()
		return
	}

	// 否则返回对方的临时公钥
	var _remEphPub = trs.FirstValue().([32]byte)
	return &_remEphPub, nil
}

// deriveSecrets 提取接收秘密值（recvSecret）和发送秘密值（sendSecret）
// 		.recvAead, err := chacha20poly1305.New(recvSecret[:])
// 		.sendAead, err := chacha20poly1305.New(sendSecret[:])
func deriveSecrets(
	dhSecret *[32]byte,
	locIsLeast bool,
) (recvSecret, sendSecret *[aeadKeySize]byte) {
	hash := sha256.New
	hkdf := hkdf.New(hash, dhSecret[:], nil, secretConnKeyAndChallengeGen)
	// res 的长度为 96 字节，其中前 64 字节用来生成 sendAead 和 recvAead，后 32 字节作为挑战值
	res := new([2*aeadKeySize + 32]byte)
	_, err := io.ReadFull(hkdf, res[:])
	if err != nil {
		panic(err)
	}

	recvSecret = new([aeadKeySize]byte)
	sendSecret = new([aeadKeySize]byte)

	if locIsLeast {
		// 如果本地临时公钥小于对方的临时公钥，则前 32 字节作为接收秘密值，中间 32 字节作为发送秘密值
		copy(recvSecret[:], res[0:aeadKeySize])
		copy(sendSecret[:], res[aeadKeySize:aeadKeySize*2])
	} else {
		// 如果本地临时公钥大于对方的临时公钥，则前 32 字节作为发送秘密值，中间 32 字节作为接收秘密值
		copy(sendSecret[:], res[0:aeadKeySize])
		copy(recvSecret[:], res[aeadKeySize:aeadKeySize*2])
	}
	return
}

// computeDHSecret 用我们自己的私钥和对方的公钥计算一个Diffie-Hellman共享密钥
func computeDHSecret(remPubKey, locPrivKey *[32]byte) (*[32]byte, error) {
	shrKey, err := curve25519.X25519(locPrivKey[:], remPubKey[:])
	if err != nil {
		return nil, err
	}
	var shrKeyArray [32]byte
	copy(shrKeyArray[:], shrKey)
	return &shrKeyArray, nil
}

// sort32 输入两个大小不一样的长度为 32 字节的数组，返回两个数组，
// 返回的第一数组为小数组，第二个数组为大数组
func sort32(foo, bar *[32]byte) (lo, hi *[32]byte) {
	// Compare 返回一个整数，按字典顺序比较两个字节片。如果 a=b，结果为 0，如果a < b，结果为-1；
	// 如果 a > b，结果为 +1；一个 nil 参数等价于一个空切片。
	if bytes.Compare(foo[:], bar[:]) < 0 {
		lo = foo
		hi = bar
	} else {
		lo = bar
		hi = foo
	}
	return
}

// signChallenge 用私钥给挑战值进行签名
func signChallenge(challenge *[32]byte, locPrivKey crypto.PrivKey) ([]byte, error) {
	signature, err := locPrivKey.Sign(challenge[:])
	if err != nil {
		return nil, err
	}
	return signature, nil
}

type authSigMessage struct {
	Key crypto.PubKey
	Sig []byte
}

// shareAuthSignature 将我们的公钥与用私钥签名的签名消息发送给对方，同时接收对方的公钥与签名消息
// 返回对方的验证消息，即对方发过来的公钥和签名消息
func shareAuthSignature(sc io.ReadWriter, pubKey crypto.PubKey, signature []byte) (recvMsg authSigMessage, err error) {

	// 将我们的公钥与用私钥签名的签名消息发送给对方，同时接收对方的公钥与签名消息
	var trs, _ = async.Parallel(
		func(_ int) (val interface{}, abort bool, err error) {
			pbpk, err := cryptoenc.PubKeyToProto(pubKey)
			if err != nil {
				return nil, true, err
			}
			_, err = protoio.NewDelimitedWriter(sc).WriteMsg(&protogossip.AuthSigMessage{PubKey: pbpk, Sig: signature})
			if err != nil {
				return nil, true, err // abort
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			var pba protogossip.AuthSigMessage
			_, err = protoio.NewDelimitedReader(sc, 1024*1024).ReadMsg(&pba)
			if err != nil {
				return nil, true, err // abort
			}

			pk, err := cryptoenc.PubKeyFromProto(pba.PubKey)
			if err != nil {
				return nil, true, err // abort
			}

			_recvMsg := authSigMessage{
				Key: pk,
				Sig: pba.Sig,
			}
			return _recvMsg, false, nil
		},
	)

	// If error:
	if trs.FirstError() != nil {
		err = trs.FirstError()
		return
	}

	var _recvMsg = trs.FirstValue().(authSigMessage)
	return _recvMsg, nil
}

//--------------------------------------------------------------------------------

// incrNonce 提取 nonce 的后 8 个字节，然后累加，累加完以后，再放回去
func incrNonce(nonce *[aeadNonceSize]byte) {
	counter := binary.LittleEndian.Uint64(nonce[4:])
	if counter == math.MaxUint64 {
		// Terminates the session and makes sure the nonce would not re-used.
		panic("can't increase nonce without overflow")
	}
	counter++
	binary.LittleEndian.PutUint64(nonce[4:], counter)
}
