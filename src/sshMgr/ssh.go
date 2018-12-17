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

type _server struct {
	Status string `json:"status,omitempty"`
	Addr   string `json:"addr"`
	Port   string `json:"port,omitempty"`
	User   string `json:"user,omitempty"`
	Passwd string `json:"passwd,omitempty"`
	Key    string `json:"key,omitempty"`
}

type _conf struct {
	User     string              `json:"user,omitempty"`
	Passwd   string              `json:"passwd,omitempty"`
	Key      string              `json:"key,omitempty"`
	Commands map[string][]string `json:"commands"`

	Servers map[string]_server `json:"servers"`
}

var (
	process = make(chan int, runtime.NumCPU())
	wg      sync.WaitGroup
	conf    _conf
)

func ParseConf(filepath string) {
	confs, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatalf("unable to read json configure file : %v", err)
	}

	err = json.Unmarshal(confs, &conf)

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

func connectTo(server _server) (*ssh.Client, error) {

	var config *ssh.ClientConfig

	if len(server.Passwd) == 0 {
		key, err := ioutil.ReadFile(GetDefault(server.Key, conf.Key))
		if err != nil {
			log.Printf("unable to read private key: %v \n %s\n", err, GetDefault(server.Key, conf.Key))
			return nil, err
		}

		// Create the Signer for this private key.
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Printf("unable to parse private key: %v\n %v\n", err, server)
			return nil, err
		}

		config = &ssh.ClientConfig{
			User: GetDefault(server.User, conf.User),
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		config = &ssh.ClientConfig{
			User: GetDefault(server.User, conf.User),
			Auth: []ssh.AuthMethod{
				ssh.Password(GetDefault(server.Passwd, conf.Passwd)),
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

func writePassword(in io.WriteCloser, out io.Reader, server _server) {

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
			log.Printf("[%s:%s] %s: %s\n", server.Addr, GetDefault(server.Port, "22"), GetDefault(server.User, conf.User), line)
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

func runCommand(server _server, cmds []string) {
	process <- 1
	defer func() { <-process; wg.Done() }()

	client, err := connectTo(server)

	if err != nil {
		return
	}

	session, err := client.NewSession()

	if err != nil {
		log.Printf("[%s:%s]: %v\n", client.RemoteAddr(), client.RemoteAddr(), err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	err = session.RequestPty("tty", 80, 40, modes)
	if err != nil {
		log.Printf("[%s:%s]: %v\n", client.RemoteAddr(), client.RemoteAddr(), err)
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

	go writePassword(in, out, server)

	session.Output(strings.Join(cmds, " && "))
}

func getServers(servername []string) (servers map[string]_server) {
	if len(servername) == 0 {
		return conf.Servers
	}

	for _, servername := range servername {
		if server, ok := conf.Servers[servername]; ok {
			servers[servername] = server
		}
	}

	return
}

func RunCommand(servername []string, cmdstr string) {

	cmds, ok := conf.Commands[cmdstr]
	if !ok {
		cmds = []string{cmdstr}
	}

	for name, server := range getServers(servername) {
		if "shutdown" == server.Status {
			log.Println(name + " configured to shutdown")
			continue
		}

		wg.Add(1)
		go runCommand(server, cmds)
	}

	wg.Wait()
}

func getValidFiles(files []string) map[string]string {
	vaildFiles := make(map[string]string)

	for _, file := range files {
		fi, err := os.Stat(file)
		if err == nil {
			vaildFiles[fi.Name()] = file
		}
	}

	return vaildFiles
}

func sendFilesToServer(server _server, files map[string]string) {
	process <- 1
	defer func() { <-process; wg.Done() }()

	client, err := connectTo(server)

	if err != nil {
		log.Printf("[%s:%s]: %v\n", client.RemoteAddr(), client.RemoteAddr(), err)
		return
	}

	for name, file := range files {

		session, err := client.NewSession()

		if err != nil {
			log.Printf("[%s:%s]: %v\n", client.RemoteAddr(), client.RemoteAddr(), err)
			return
		}

		err = scp.CopyPath(file, name, session)
		if err != nil {
			log.Println(err)
			continue
		}

		log.Printf("[%s:%s]: send file (%s) success !\n", client.RemoteAddr(), client.RemoteAddr(), file)
	}
}

func SendFiles(servername []string, files []string) {
	vfs := getValidFiles(files)

	for name, server := range getServers(servername) {
		if "shutdown" == server.Status {
			log.Println(name + " configured to shutdown")
			continue
		}

		wg.Add(1)
		go sendFilesToServer(server, vfs)
	}

	wg.Wait()
}
