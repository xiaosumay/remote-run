# remote-run
```batch
Usage:
  remote-run.exe [OPTIONS]

Application Options:
  /f, /conf:    指定配置文件
  /c, /script:  运行命令或bash文件 例如：netstat -lntp
  /s, /server:  指定运行命令的服务器， 默认是全部服务器
  /b, /build    生成一个初始化的配置文件server.json
  /u, /upload:  上传一个文件到服务器上
      /dest:    文件存放的路径，默认为$HOME目录
  /a, /add      添加一个远程服务器
  /m, /mod      修改一个远程服务器
  /d, /delete   删除一个远程服务器
  /l, /list     列出配置中所有服务器
      /name:    仅与/a,/m,/d 使用；服务器的名字，识别用
      /addr:    仅与/a,/m,/d 使用；服务器地址，可以是IP也可以是域名
      /user:    仅与/a,/m,/d 使用；登陆用户名
      /port:    仅与/a,/m,/d 使用；服务器ssh端口，默认22
      /passwd:  仅与/a,/m,/d 使用；若为空，则使用主密码，若主密码还为空，则使用key
      /key:     仅与/a,/m,/d 使用；登陆的publickey
      /status:  仅与/a,/m,/d 使用；服务器状态，默认为空, shutdown时不执行任何命令

Help Options:
  /?            Show this help message
  /h, /help     Show this help message
```
### 配置服务器
- 方式一：打开文件servers.json并编辑
- 方式二：使用remote-run工具

详解方式二：
1. 添加服务器：
`remote-run -a --name test_server_001 --addr 127.0.0.1 --user root --passwd 112233`

2. 删除服务器：
`remote-run -d --name test_server_001`

3. 修改服务器：
    ```bash
    remote-run -m --name test_server_001 --addr 127.0.0.1
    remote-run -m --name test_server_001 --user admin
    remote-run -m --name test_server_001 --passwd 223344
    remote-run -m --name test_server_001 --status shutdown
    ```

### 上传文件
`remote-run -u ./nginx.sh` (仅上传文件，与下面执行文件不一样)

###  执行命令
- 直接执行命令
`remote-run -c "echo $(hostname)"`

- 执行文件
`remote-run -c ./nginx.sh`

- 执行单一服务器命令
`remote-run -s test_server_001  ...`

###  生成默认配置
在当前文件夹下生成默认配置
`remote-run -b`

###  查看当前受影响的配置文件
`remote-run -l`

###  指定配置文件
`remote-run -f my_servers.json ...`
