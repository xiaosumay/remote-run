package main

import (
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	"sshMgr"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var opts struct {
		ConfPath   string   `short:"f" long:"conf" description:"server configures file"`
		Command    string   `short:"c" long:"script" description:"run script by global defined"`
		ServerName []string `short:"s" long:"server" description:"run script only in server list, default ALL"`
		BuildConf  bool     `long:"build" description:"build a default server.json"`
		UploadFile []string `short:"u" long:"upload" description:"upload files"`
	}

	var parser = flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()

	if err != nil {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	if opts.BuildConf {
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
            "passwd": "112233",
			"key": "C:\\id_rsa"
        }
    }
}`
		err := ioutil.WriteFile(sshMgr.GetDefault(opts.ConfPath, "servers.json"), []byte(defaultJson), os.ModePerm)

		if err != nil {
			log.Fatalln(err)
		}

		return
	}

	sshMgr.ParseConf(sshMgr.GetDefault(opts.ConfPath, "servers.json"))

	if len(opts.UploadFile) != 0 {
		sshMgr.SendFiles(opts.ServerName, opts.UploadFile)
	}

	if len(opts.Command) != 0 {
		sshMgr.RunCommand(opts.ServerName, opts.Command)
	}
}
