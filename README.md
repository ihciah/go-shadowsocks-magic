# go-shadowsocks-magic

A shadowsocks implementation in golang with Multi-connection Acceleration.

The code is based on https://github.com/shadowsocks/go-shadowsocks2

==================================================

## UPDATE 2019-10: Deprecated. [Rabbit TCP]( https://github.com/ihciah/rabbit-tcp ) is recommended.

## 2019-10 更新: 不建议使用。请使用[Rabbit TCP]( https://github.com/ihciah/rabbit-tcp ) 。

==================================================

[中文版戳这！](https://www.ihcblog.com/How-to-Speed-Up-Shadowsocks/)



## Features

- [x] SOCKS5 proxy with UDP Associate
- [x] Support for Netfilter TCP redirect (IPv6 should work but not tested)
- [x] UDP tunneling (e.g. relay DNS packets)
- [x] TCP tunneling (e.g. benchmark with iperf3)
- [x] Multi-connection Acceleration


## Multi-connection Acceleration Protocol
In the original shadowsocks protocal, the data is transmitted in the following way:

`client <---> ss-local <--[encrypted]--> ss-remote <---> target`

The shadowsocks is used to access blocked servers.
In most common applications, the bottleneck of the bandwidth is the way outside their countries: `ss-local<--->ss-remote`.
Is it the best way to communicate in one connection? 

With a proper protocol, the `ss-local` and `ss-remote` can communicate in multi TCP connections, 
which will be faster when transferring a large amount of data, especially for those slow VPS.

The following protocol by ihciah is one of them. The implementation in other language is welcome.

1. Establish the connection to shadowsocks server first.
2. Send [address](https://shadowsocks.org/en/spec/Protocol.html). 
Here I add 2 bits for "Magic": `magic-main`(`0b01000`) and `magic-child`(`0b10000`).

    After sending the address with `magic-main`, the server will reply a 16-byte `dataKey`.
3. Up till now we have 1 connection. 
Through the connection, data will be sent back in format `[BlockID(uint32)][BlockSize(uint32)][Data([BlckSize]byte)]`.
4. Of course, it's not acceleration with only 1 connection. 
With the `dataKey` we can construct an "address" in format `[Type([1]byte)][dataKey([16]byte)]`, 
the type must be marked as `magic-child`.

    Now create a new connection and send the 17-byte "address", 
    then the data will be transmitted to the client.
5. Once the main connection(the first one) is disconnected, 
all the other connections will die. 
However, it's not a problem if the `magic-child` connections interrupted.
   


## Install

Pre-built binaries for common platforms are available at https://github.com/ihciah/go-shadowsocks-magic/releases

Install from source

```sh
go get -u -v github.com/ihciah/go-shadowsocks-magic
```


## Basic Usage

### Server

Start a server listening on port 8488 using `RC4-MD5` cipher with password `your-password`.

```sh
shadowsocks-magic -s 'ss://RC4-MD5:your-password@:8488' -verbose
```

When deploy, you can close verbose.

### Client

Start a client connecting to the above server. The client listens on port 1080 for incoming SOCKS5 
connections, and tunnels both UDP and TCP on port 8053 and port 8054 to 8.8.8.8:53 and 8.8.4.4:53 
respectively. 

```sh
shadowsocks-magic -c 'ss://RC4-MD5:your-password@[server_address]:8488' \
    -verbose -socks :1080 -u -udptun :8053=8.8.8.8:53,:8054=8.8.4.4:53 \
                             -tcptun :8053=8.8.8.8:53,:8054=8.8.4.4:53
```

Replace `[server_address]` with the server's public address.


## Advanced Usage


### Netfilter TCP redirect (Linux only)

The client offers `-redir` and `-redir6` (for IPv6) options to handle TCP connections 
redirected by Netfilter on Linux. The feature works similar to `ss-redir` from `shadowsocks-libev`.


Start a client listening on port 1082 for redirected TCP connections and port 1083 for redirected
TCP IPv6 connections.

```sh
shadowsocks-magic -c 'ss://RC4-MD5:your-password@[server_address]:8488' -redir :1082 -redir6 :1083
```


### TCP tunneling

The client offers `-tcptun [local_addr]:[local_port]=[remote_addr]:[remote_port]` option to tunnel TCP.
For example it can be used to proxy iperf3 for benchmarking.

Start iperf3 on the same machine with the server.

```sh
iperf3 -s
```

By default iperf3 listens on port 5201.

Start a client on the same machine with the server. The client listens on port 1090 for incoming connections
and tunnels to localhost:5201 where iperf3 is listening.

```sh
shadowsocks-magic -c 'ss://RC4-MD5:your-password@[server_address]:8488' -tcptun :1090=localhost:5201
```

Start iperf3 client to connect to the tunneld port instead

```sh
iperf3 -c localhost -p 1090
```


## Design Principles

The code base strives to

- be idiomatic Go and well organized;
- use fewer external dependences as reasonably possible;
- only include proven modern ciphers;

## Known Issues

