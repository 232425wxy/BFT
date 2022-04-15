#!/usr/bin/env sh

##
## Input parameters
##
BINARY=/bft/${BINARY:-bft}
ID=${ID:-0}
LOG=${LOG:-bft.log}

##
## Assert linux binary
##



if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder."
	exit 1
fi

BINARY_CHECK="$(file "$BINARY" | grep 'ELF 64-bit LSB executable, x86-64')"
if [ -z "${BINARY_CHECK}" ]; then
	echo "Binary needs to be OS linux, ARCH amd64"
	exit 1
fi

##
## Run binary with all parameters
##
export CRHOME="/bft/node${ID}"

if [ -d "`dirname ${CRHOME}/${LOG}`" ]; then
	echo "${CRHOME}/${LOG} exist..."
  "$BINARY" "$@" | tee "${CRHOME}/${LOG}"
else
	echo "${CRHOME}/${LOG} doesn't exist!!!"
  "$BINARY" "$@"
fi

chmod 777 -R /bft

