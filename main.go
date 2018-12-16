package main

import (
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	. "sshMgr"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var opts struct {
		ConfPath   string   `short:"f" long:"conf" description:"server configures file"`
		Command    string   `short:"c" long:"script" description:"run script by global defined"`
		ServerName []string `short:"s" long:"server" description:"run script only in server list, default ALL"`
		BuildConf  bool     `long:"build" description:"build a default server.json"`
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
		err := ioutil.WriteFile("servers.json", []byte(defaultJson), os.ModePerm)

		if err != nil {
			log.Fatalln(err)
		}

		return
	}

	SshMgr.ParseConf(GetDefault(opts.ConfPath, "servers.json"))

	if cmd, ok := SshMgr.Conf.Commands[opts.Command]; ok {
		SshMgr.RunCommand(opts.ServerName, cmd)
	} else {
		SshMgr.RunCommand(opts.ServerName, []string{opts.Command})
	}
}
