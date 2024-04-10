## pTunnel
### 背景
学校的服务器非常封闭，甚至连 SSH 端口都无法直接连接，每次都需要挂 VPN 才能访问实验室的服务器。为了方便访问实验室的服务器，
最初使用的是[frp](https://github.com/fatedier/frp)进行内网穿透。但是好景不长，学校服务器的防火墙识别出了frp的请求，
然后将其拦截了。为了继续进行内网穿透，我决定自己手撸一个内网穿透工具。

### 简介
pTunnel 是一个内网穿透工具，通过 pTunnel 可以将内网的 TCP 服务映射到公网上，从而可以通过公网访问内网的服务。
该项目参考自[frp v0.5.0](https://github.com/fatedier/frp/tree/v0.5.0)。加密通信的机制参考自https加
密过程。首先客户端生成一个AES密钥, 然后将AES密钥通过服务器的公钥加密，发送给服务器。服务器使用RSA私钥解密，得
到AES密钥，后续通信则会使用该AES密钥加密通信数据。与https不同的是，公钥直接保存在了服务器，而不是通过CA机构来
签发。
第二点，pTunnel在建立内网穿透的隧道时，支持多种隧道类型，包括TCP隧道、KCP隧道、SSH隧道。每种隧道都支持数据通过
生成的AES密钥进行加密（当然也可以不加密）。

### 快速开始
1. 获取 pTunnelClient 和 pTunnelServer
   1. 从源码编译(需要安装 go 环境)
       1. 服务器端
           ```shell
           git clone https://github.com/pangxincheng/pTunnel.git
           cd pTunnel
           go mod tidy
           go build -o pTunnelServer cmd/server/pTunnelServer.go
           go build -o pTunnelGenRSAKey cmd/genRSAKey/pTunnelGenRSAKey.go
           ```
       2. 客户端
           ```shell
           git clone https://github.com/pangxincheng/pTunnel.git
           cd pTunnel
           go mod tidy
           go build -o pTunnelClient cmd/client/pTunnelClient.go
           ```
   2. 从 release 下载

2. 在服务器上执行下面的命令，生成 RSA 公钥和密钥
    ```shell
    ./pTunnelGenRSAKey
    ```
3. 将生成的`cert/PublicKey.pem`和`cert/NBits.txt`文件拷贝到客户端目录下
4. 在服务器端配置`conf/server.ini`文件
    ```ini
    [common]
    ; RSA私钥文件路径
    PrivateKeyFile = cert/PrivateKey.pem
    ; RSA公钥位数文件路径
    NBitsFile = cert/NBits.txt
    ; 服务器端监听地址
    ServerPort = 7000
    ; 日志文件路径，console表示输出到控制台
    LogFile = console
    ; 日志级别，debug/info/warn/error
    LogLevel = info
    ; 日志文件最大保存天数
    LogMaxDays = 3
    ; 心跳超时时间，单位为秒
    HeartbeatTimeout = 10
   ```
5. 在客户端配置`conf/client.ini`文件
    ```ini
    [common]
    ; 服务器端地址
    ServerAddr = localhost
    ; 需要与服务器端配置的ServerPort一致
    ServerPort = 7000
    ; RSA公钥文件路径
    PublicKeyFile = cert/PublicKey.pem
    ; RSA公钥位数文件路径
    NBitsFile = cert/NBits.txt
    ; 日志文件路径，console表示输出到控制台
    LogFile = console
    ; 日志级别，debug/info/warn/error
    LogLevel = info
    ; 日志文件最大保存天数
    LogMaxDays = 3
    
    ; 需要映射的内网服务, 后面的配置项可以有多个
    [ssh]
    ; 内网服务的地址
    InternalAddr = localhost
    ; 内网服务的端口
    InternalPort = 22
    ;内网服务的类型，目前只支持tcp
    InternalType = tcp
    ;映射到公网的端口
    ExternalPort = 2222
    ;映射到公网的类型，目前只支持tcp
    ExternalType = tcp
    ; 隧道的类型，可以是tcp、kcp、ssh
    TunnelType = tcp
    ; 是否加密隧道 true/false(ssh隧道建议设置为false, 因为ssh本身就是加密的)
    TunnelEncrypt = false
   ```

6. 此时你的项目目录应该至少包含以下文件
    ```shell
    # 服务器端
    .
    ├── cert
    │   ├── NBits.txt
    │   └── PublicKey.pem
    |   └── PrivateKey.pem
    ├── conf
    │   └── server.ini
    ├── pTunnelServer
    └── pTunnelGenRSAKey
   
    # 客户端
    .
    ├── cert
    │   ├── NBits.txt
    │   └── PublicKey.pem
    ├── conf
    │   └── client.ini
    └── pTunnelClient
    ```
7. 在服务器端执行下面的命令
    ```shell
    ./pTunnelServer
    ```

8. 在客户端执行下面的命令
    ```shell
    ./pTunnelClient
    ```

9. 此时你就可以通过访问服务器端的`2222`端口访问内网的SSH服务了
### TODO
1. ~~支持数据加密隧道~~
2. ~~支持SSH隧道~~
3. ~~README.md的完善~~
4. 更复杂的隧道验证机制
5. 服务内部的状态监控