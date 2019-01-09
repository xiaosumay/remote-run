package main

import (
	"bufio"
	"bytes"
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

func myJSONMarshal(t interface{}, indent bool) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if indent {
		encoder.SetIndent("", "    ")
	}
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

func MgrConf(filepath string, isAdd, isMod, isDel, isList bool, opts []string) {
	ParseConf(filepath)

	if isList {
		data, err := myJSONMarshal(conf.Servers, true)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(string(data))
		return
	}

	if isAdd || isMod {
		if len(opts[0]) == 0 {
			log.Fatalln("name must be set")
		}

		if server, ok := conf.Servers[opts[0]]; ok {
			opts[1] = GetDefault(opts[1], server.Addr)
			opts[2] = GetDefault(opts[2], server.Port)
			opts[3] = GetDefault(opts[3], server.Passwd)
			opts[4] = GetDefault(opts[4], server.Key)
			opts[5] = GetDefault(opts[5], server.Status)
			opts[6] = GetDefault(opts[6], server.User)
		}

		conf.Servers[opts[0]] = _server{
			Addr:   opts[1],
			Port:   opts[2],
			Passwd: opts[3],
			Key:    opts[4],
			Status: opts[5],
			User:   opts[6],
		}
	} else if isDel {
		if _, ok := conf.Servers[opts[0]]; ok {
			delete(conf.Servers, opts[0])
		}
	}

	data, err := myJSONMarshal(conf, true)

	if err != nil {
		log.Fatalln(err)
	}

	err = ioutil.WriteFile(filepath, data, os.ModePerm)

	if err != nil {
		log.Fatalln(err)
	}
}

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

func isEmpty(str string) bool {
	return 0 == len(str)
}

func connectTo(server _server) (*ssh.Client, error) {

	var config *ssh.ClientConfig

	if isEmpty(server.Passwd) && (!isEmpty(server.Key) || isEmpty(conf.Passwd)) {
		keyStr := GetDefault(server.Key, conf.Key)
		key, err := ioutil.ReadFile(keyStr)

		if err != nil {
			log.Printf("unable to read private key: %v \n %s\n", err, keyStr)
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

	ipaddr := fmt.Sprintf("%s:%s", strings.Trim(server.Addr, " "), GetDefault(server.Port, "22"))
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
			log.Printf("[%s:%s] %s: %v\n", server.Addr, GetDefault(server.Port, "22"), GetDefault(server.User, conf.User), line)
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

		if strings.HasSuffix(line, "[Y/n] ") || strings.HasSuffix(line, "[y/N]: ") {
			_, err := in.Write([]byte("y\n"))
			if err != nil {
				break
			}
		}
	}
}

func runCommand(name string, server _server, cmds []string) {
	process <- 1
	defer func() { <-process; wg.Done() }()

	client, err := connectTo(server)

	if err != nil {
		return
	}

	session, err := client.NewSession()

	if err != nil {
		log.Printf("[%s]: %v\n", client.RemoteAddr(), err)
		return
	}

	for idx, cmd := range cmds {
		fi, err := os.Stat(cmd)
		session2, err2 := client.NewSession()
		if err == nil && err2 == nil {
			if err = scp.CopyPath(cmd, fi.Name(), session2); err == nil {
				cmds[idx] = "bash " + fi.Name()
			} else {
				cmds = append(cmds[:idx], cmds[idx+1:]...)
			}
		}
		session2.Close()
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

	session.Output("export SERVER_NAME=" + name + "; " + strings.Join(cmds, " && "))
}

func getServers(servername []string) map[string]_server {
	if len(servername) == 0 {
		return conf.Servers
	}

	var servers = make(map[string]_server)

	for _, servername := range servername {
		if server, ok := conf.Servers[servername]; ok {
			servers[servername] = server
		}
	}

	return servers
}

func RunCommand(servername []string, cmd string) {

	cmds, ok := conf.Commands[cmd]
	if !ok {
		cmds = []string{cmd}
	}

	for name, server := range getServers(servername) {
		if "shutdown" == server.Status {
			log.Printf("server [%s] configured to shutdown", name)
			continue
		}

		wg.Add(1)
		go runCommand(name, server, cmds)
	}

	wg.Wait()
}

func getValidFiles(files []string) map[string]string {
	validFiles := make(map[string]string)

	for _, file := range files {
		fi, err := os.Stat(file)
		if err == nil {
			validFiles[fi.Name()] = file
		}
	}

	return validFiles
}

func sendFilesToServer(server _server, files map[string]string, destPath string) {
	process <- 1
	defer func() { <-process; wg.Done() }()

	client, err := connectTo(server)

	if err != nil {
		log.Printf("[%s]: %v\n", server.Addr, err)
		return
	}

	if 0 != len(destPath) {
		session, err := client.NewSession()

		if err != nil {
			log.Printf("[%s]: %v\n", server.Addr, err)
			session.Close()
			return
		}

		destPath = strings.Trim(destPath, "'")
		destPath = strings.Trim(destPath, "\"")

		err = session.Run("sudo mkdir -p " + destPath + " && sudo chown `whoami`:`whoami` " + destPath)
		if err != nil {
			log.Fatalf("[%s]: %v\n", server.Addr, err)
			return
		}

		session.Close()

		if !strings.HasSuffix(destPath, "/") {
			destPath += "/"
		}
	}

	for name, file := range files {
		session, err := client.NewSession()

		if err != nil {
			log.Printf("[%s]: %v\n", server.Addr, err)
			session.Close()
			return
		}

		err = scp.CopyPath(file, destPath+name, session)
		if err != nil {
			log.Println(err)
			session.Close()
			continue
		}

		log.Printf("[%s]: send file (%s) success !\n", client.RemoteAddr(), file)
		session.Close()
	}
}

func SendFiles(servername []string, files []string, destPath string) {
	vfs := getValidFiles(files)

	for name, server := range getServers(servername) {
		if "shutdown" == server.Status {
			log.Println(name + " configured to shutdown")
			continue
		}

		wg.Add(1)
		go sendFilesToServer(server, vfs, destPath)
	}

	wg.Wait()
}
