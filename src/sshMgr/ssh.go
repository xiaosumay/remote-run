package sshMgr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/tmc/scp"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
)

type server struct {
	Status string `json:"status,omitempty"`
	Addr   string `json:"addr"`
	Port   string `json:"port,omitempty"`
	User   string `json:"user,omitempty"`
	Passwd string `json:"passwd,omitempty"`
	Key    string `json:"key,omitempty"`
}

type conf struct {
	User     string              `json:"user,omitempty"`
	Passwd   string              `json:"passwd,omitempty"`
	Key      string              `json:"key,omitempty"`
	Commands map[string][]string `json:"commands"`

	Servers map[string]server `json:"servers"`
}

type SSH struct {
	Conf conf
}

var (
	process = make(chan int, runtime.NumCPU())
	wg      sync.WaitGroup
	SshMgr  = new(SSH)
)

func (thiz *SSH) ParseConf(filepath string) {
	confs, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatalf("unable to read json configure file : %v", err)
	}

	err = json.Unmarshal(confs, &thiz.Conf)

	if err != nil {
		log.Fatalf("unable parse json file : %v", err)
	}
}

func GetDefault(key, fallback string) string {
	if len(key) != 0 {
		return key
	}
	return fallback
}

func (thiz *SSH) connectTo(server server) (*ssh.Client, error) {

	var config *ssh.ClientConfig

	if len(server.Passwd) == 0 {
		key, err := ioutil.ReadFile(GetDefault(server.Key, thiz.Conf.Key))
		if err != nil {
			log.Printf("unable to read private key: %v \n %s\n", err, GetDefault(server.Key, thiz.Conf.Key))
			return nil, err
		}

		// Create the Signer for this private key.
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Printf("unable to parse private key: %v\n %v\n", err, server)
			return nil, err
		}

		config = &ssh.ClientConfig{
			User: GetDefault(server.User, thiz.Conf.User),
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		config = &ssh.ClientConfig{
			User: GetDefault(server.User, thiz.Conf.User),
			Auth: []ssh.AuthMethod{
				ssh.Password(GetDefault(server.Passwd, thiz.Conf.Passwd)),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	ipaddr := fmt.Sprintf("%s:%s", server.Addr, GetDefault(server.Port, "22"))
	client, err := ssh.Dial("tcp", ipaddr, config)

	if err != nil {
		log.Printf("unable to connect: %v\n", err)
		return nil, err
	}

	return client, err
}

func (thiz *SSH) writePassword(in io.WriteCloser, out io.Reader, server server) {

	wg.Add(1)
	defer wg.Done()

	var (
		line string
		r    = bufio.NewReader(out)
	)

	for {
		b, err := r.ReadByte()
		if err != nil {
			break
		}

		if b == byte('\n') {
			line = strings.TrimLeft(line, "\n")
			log.Printf("[%s:%s] %s: %s\n", server.Addr, GetDefault(server.Port, "22"), GetDefault(server.User, thiz.Conf.User), line)
			line = ""
			continue
		}

		line += string(b)

		if strings.HasPrefix(line, "[sudo] password for ") && strings.HasSuffix(line, ": ") {
			_, err := in.Write([]byte(server.Passwd + "\n"))
			if err != nil {
				break
			}
		}
	}
}

func (thiz *SSH) runCommand(server server, cmds []string) {
	process <- 1
	defer func() { <-process; wg.Done() }()

	client, err := thiz.connectTo(server)

	if err != nil {
		return
	}

	session, err := client.NewSession()

	if err != nil {
		log.Printf("[%s:%s]: %v\n", server.Addr, GetDefault(server.Port, "22"), err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	err = session.RequestPty("tty", 80, 40, modes)
	if err != nil {
		log.Printf("[%s:%s]: %v\n", server.Addr, GetDefault(server.Port, "22"), err)
		return
	}

	for idx, cmdstr := range cmds {
		fi, err := os.Stat(cmdstr)
		session2, errr := client.NewSession()
		if err == nil && errr == nil {
			if err = scp.CopyPath(cmdstr, fi.Name(), session2); err == nil {
				cmds[idx] = "bash " + fi.Name()
			} else {
				cmds = append(cmds[:idx], cmds[idx+1:]...)
			}
		}
	}

	in, err := session.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	out, err := session.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	go thiz.writePassword(in, out, server)

	session.Output(strings.Join(cmds, " && "))
}

func (thiz *SSH) RunCommand(servername []string, cmds []string) {

	if len(servername) == 0 {
		for name, server := range thiz.Conf.Servers {
			if "shutdown" == server.Status {
				log.Println(name + " configured to shutdown")
				continue
			}
			wg.Add(1)
			go thiz.runCommand(server, cmds)
		}
	} else {
		for _, servername := range servername {
			var server server
			var ok bool
			if server, ok = thiz.Conf.Servers[servername]; !ok {
				continue
			}

			if "shutdown" == server.Status {
				log.Println(servername + " configured to shutdown")
				continue
			}

			wg.Add(1)
			go thiz.runCommand(server, cmds)
		}

	}

	wg.Wait()
}
