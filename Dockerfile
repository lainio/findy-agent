FROM optechlab/indy-golang:1.15.0

WORKDIR /go/src/github.com/findy-network/findy-agent

ADD .docker/findy-wrapper-go /go/src/github.com/findy-network/findy-wrapper-go
ADD . .

RUN make deps && make install

FROM optechlab/indy-base:1.15.0

ADD ./tools/start-server.sh /start-server.sh

COPY --from=0 /go/bin/findy-agent /findy-agent

RUN echo "{}" > /root/findy.json

EXPOSE 8080

ENV HOST_ADDR localhost
ENV REGISTRY_PATH /root/findy.json
ENV PSMDB_PATH /root/findy.bolt
ENV FINDY_AGENT_CERT_PATH /aps.p12

CMD ["/start-server.sh", "/"]