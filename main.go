package main

import (
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sshMgr"
)

func GetExePath() string {
	file, _ := exec.LookPath(os.Args[0])

	//得到全路径，比如在windows下E:\\golang\\test\\a.exe
	path, _ := filepath.Abs(file)

	rst := filepath.Dir(path)

	return rst
}

func BuildDefaultConf(path string) {
	defaultJson := `{
    "user": "root",
    "key": "C:\\id_rsa",
	"passwd": "112233",
    "commands": {
        "test": [
            "echo \"Hello $(hostname)!\""
        ],
        "zsh": [
            "[[ ! \"$(ps -p $$ -oargs=)\" =~ \"zsh\" ]]",
            "wget https://raw.githubusercontent.com/xiaosumay/shellscripts/master/extras/zsh.run -O zsh.run",
            "sudo bash zsh.run"
        ],
        "lnmp": [
            "! type lnmp >/dev/null 2>&1",
            "wget https://raw.githubusercontent.com/xiaosumay/shellscripts/master/extras/lnmp72.run -O lnmp72.run",
            "sudo screen -UdmS lnmp zsh lnmp72.run"
        ],
        "redis": [
            "! type lnmp >/dev/null 2>&1",
            "cd lnmp1.5 && ./addon.sh install redis"
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

func main() {

	//log.SetFlags(log.LstdFlags | log.Lshortfile)

	var opts struct {
		ConfPath   string   `short:"f" long:"conf" description:"server configures file"`
		Command    string   `short:"c" long:"script" description:"run script by global defined"`
		ServerName []string `short:"s" long:"server" description:"run script only in server list, default ALL"`
		BuildConf  bool     `short:"b" long:"build" description:"build a default server.json"`
		UploadFile []string `short:"u" long:"upload" description:"upload files before run script"`

		AddServer    bool `short:"a" long:"add" description:"add server to configures file"`
		ModServer    bool `short:"m" long:"mod" description:"modify server to configures file"`
		DeleteServer bool `short:"d" long:"delete" description:"delete server to configures file"`
		ListServer   bool `short:"l" long:"list" description:"list servers"`

		Name   string `long:"name" description:"server tag. -a or -m or -d must be set"`
		Addr   string `long:"addr" description:"server tag. -a or -m or -d must be set"`
		User   string `long:"user" description:"server tag. -a or -m or -d must be set"`
		Port   string `long:"port" description:"server tag. -a or -m or -d must be set"`
		Passwd string `long:"passwd" description:"server tag. -a or -m or -d must be set"`
		Key    string `long:"key" description:"server tag. -a or -m or -d must be set"`
		Status string `long:"status" description:"server tag. -a or -m or -d must be set"`
	}

	var parser = flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()

	confPath := sshMgr.GetDefault(opts.ConfPath, GetExePath()+filepath.FromSlash("/servers.json"))

	for {
		if err != nil {
			break
		}

		if opts.AddServer || opts.ModServer || opts.DeleteServer || opts.ListServer {
			sshMgr.MgrConf(confPath, opts.AddServer, opts.ModServer, opts.DeleteServer, opts.ListServer, []string{
				opts.Name, opts.Addr, opts.Port, opts.Passwd, opts.Key, opts.Status, opts.User,
			})
			return
		}

		if len(opts.Command) == 0 && len(opts.UploadFile) == 0 && !opts.BuildConf {
			break
		}

		if opts.BuildConf {
			BuildDefaultConf(confPath)
		}

		sshMgr.ParseConf(confPath)

		if len(opts.UploadFile) != 0 {
			sshMgr.SendFiles(opts.ServerName, opts.UploadFile)
		}

		if len(opts.Command) != 0 {
			sshMgr.RunCommand(opts.ServerName, opts.Command)
		}

		return
	}

	parser.WriteHelp(os.Stderr)
	os.Exit(1)
}
