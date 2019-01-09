package main

import (
	"errors"
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

func BuildDefaultConf(path string) {
	defaultJson := `{
    "user": "root",
    "key": "C:\\id_rsa",
	"passwd": "112233",
    "commands": {
		"lnmp": [
            "wget https://raw.githubusercontent.com/xiaosumay/shellscripts/master/extras/lnmp72.run -O lnmp72.run",
            "sudo zsh lnmp72.run"
        ],
        "redis": [
            "! type lnmp >/dev/null 2>&1",
            "cd lnmp1.5 && ./addon.sh install redis"
        ],
        "test": [
            "echo \"Hello $(hostname)!\""
        ],
        "zsh": [
            "wget https://raw.githubusercontent.com/xiaosumay/shellscripts/master/extras/zsh.run -O zsh.run",
            "sudo bash zsh.run"
        ]
    },
    "servers": {
        "ubuntu1": {
            "status": "shutdown",
            "addr": "192.168.0.2",
			"port": "22",
            "passwd": "112233",
			"key": "C:\\id_rsa"
        }
    }
}`
	err := ioutil.WriteFile(path, []byte(defaultJson), os.ModePerm)

	if err != nil {
		log.Fatalln(err)
	}

	return
}

func GetConfig(path string) (string, error) {
	curDir, _ := os.Getwd()
	usr, _ := user.Current()
	file, _ := exec.LookPath(os.Args[0])
	absPath, _ := filepath.Abs(file)
	exePath := filepath.Dir(absPath)

	paths := []string{
		path,
		curDir + filepath.FromSlash("/servers.json"),
		usr.HomeDir + filepath.FromSlash("/.sshmgr_servers.json"),
		exePath + filepath.FromSlash("/servers.json"),
	}

	for _, realpath := range paths {
		_, err := os.Stat(realpath)
		if err == nil {
			return realpath, nil
		}
	}

	return "", errors.New("can not find file servers.json")
}

func main() {

	// log.SetFlags(log.LstdFlags | log.Lshortfile)

	var opts struct {
		ConfPath   string   `short:"f" long:"conf" description:"指定配置文件"`
		Command    string   `short:"c" long:"script" description:"运行命令或bash文件 例如：netstat -lntp"`
		ServerName []string `short:"s" long:"server" description:"指定运行命令的服务器， 默认是全部服务器"`
		BuildConf  bool     `short:"b" long:"build" description:"生成一个初始化的配置文件server.json"`
		UploadFile []string `short:"u" long:"upload" description:"上传一个文件到服务器上"`
		DestPath   string	`long:"dest" description:"文件存放的路径，默认为$HOME目录"`

		AddServer    bool `short:"a" long:"add" description:"添加一个远程服务器"`
		ModServer    bool `short:"m" long:"mod" description:"修改一个远程服务器"`
		DeleteServer bool `short:"d" long:"delete" description:"删除一个远程服务器"`
		ListServer   bool `short:"l" long:"list" description:"列出配置中所有服务器"`

		Name   string `long:"name" description:"仅与/a,/m,/d 使用；服务器的名字，识别用"`
		Addr   string `long:"addr" description:"仅与/a,/m,/d 使用；服务器地址，可以是IP也可以是域名"`
		User   string `long:"user" description:"仅与/a,/m,/d 使用；登陆用户名"`
		Port   string `long:"port" description:"仅与/a,/m,/d 使用；服务器ssh端口，默认22"`
		Passwd string `long:"passwd" description:"仅与/a,/m,/d 使用；若为空，则使用主密码，若主密码还为空，则使用key"`
		Key    string `long:"key" description:"仅与/a,/m,/d 使用；登陆的publickey"`
		Status string `long:"status" description:"仅与/a,/m,/d 使用；服务器状态，默认为空, shutdown时不执行任何命令"`
	}

	var parser = flags.NewParser(&opts, flags.Default)

	_, err := parser.Parse()

	if err != nil {
		os.Exit(1)
	}

	confPath, err := GetConfig(opts.ConfPath)

	if err != nil {
		return
	}

	log.Printf("use configure from %s\n", confPath)

	if opts.AddServer || opts.ModServer || opts.DeleteServer || opts.ListServer {
		MgrConf(confPath, opts.AddServer, opts.ModServer, opts.DeleteServer, opts.ListServer, []string{
			opts.Name, opts.Addr, opts.Port, opts.Passwd, opts.Key, opts.Status, opts.User,
		})
		return
	}

	if len(opts.Command) == 0 && len(opts.UploadFile) == 0 && !opts.BuildConf {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	if opts.BuildConf {
		BuildDefaultConf(confPath)
		return
	}

	ParseConf(confPath)

	if len(opts.UploadFile) != 0 {
		SendFiles(opts.ServerName, opts.UploadFile, opts.DestPath)
	}

	if len(opts.Command) != 0 {
		RunCommand(opts.ServerName, opts.Command)
	}
}
