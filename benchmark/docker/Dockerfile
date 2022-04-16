FROM alpine:3.7

RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq file

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

VOLUME [ /bft ]
WORKDIR /bft
EXPOSE 36656 36657 26660
ENTRYPOINT ["/usr/bin/helper.sh"]
CMD ["node", "--proxy_app", "kvstore"]
STOPSIGNAL SIGTERM

COPY helper.sh /usr/bin/helper.sh
COPY config-template.toml /etc/bft/config-template.toml
