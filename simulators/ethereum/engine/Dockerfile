# This simulation runs Engine API tests.
# FROM golang:1-alpine as builder
# RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable.
# ADD . /source
# WORKDIR /source
# RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
RUN mkdir /source
ADD ./keys /source/keys
ADD ./init /source/init
ADD ./chains /source/chains
ADD ./engine /source/engine
WORKDIR /source
RUN chmod a+x engine
# COPY --from=builder /source/engine .
ENTRYPOINT ["./engine"]