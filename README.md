# Port Knocker

[![Go](https://github.com/MatthewLavine/portknocker/actions/workflows/go.yml/badge.svg)](https://github.com/MatthewLavine/portknocker/actions/workflows/go.yml) [![Docker Image CI](https://github.com/MatthewLavine/portknocker/actions/workflows/docker-image.yml/badge.svg)](https://github.com/MatthewLavine/portknocker/actions/workflows/docker-image.yml)

Basic implementation of a port knocking server + client.

The server hosts port 8080 to an allowlist. A client must knock the ports listed by `knockLength` in sequence within `knockTimeout` to be added to the allowlist.

### Running directly

```
$ go run server/server.go &
$ go run client/client.go
$ kill %1 # When done
```

### Running with docker

```
$ docker build -t portknockserver .
$ docker run --rm -p 8080:8080 -p 8081:8081 portknockserver
```
