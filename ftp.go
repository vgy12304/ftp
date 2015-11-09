package ftp

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type FTP struct {
	host     string
	port     int
	user     string
	passwd   string
	pasv     int
	cmd      string
	Code     int
	Message  string
	Debug    bool
	stream   []byte
	conn     net.Conn
	dataConn net.Conn
	Error    error
}

func (ftp *FTP) debugInfo(s string) {
	if ftp.Debug {
		fmt.Println(s)
	}
}

func (ftp *FTP) Connect(host string, port int) {
	addr := fmt.Sprintf("%s:%d", host, port)
	ftp.conn, ftp.Error = net.Dial("tcp", addr)
	ftp.Response()
	ftp.host = host
	ftp.port = port
}

func (ftp *FTP) openDataConn(port int) {
	// Build the new net address string
	addr := net.JoinHostPort(ftp.host, strconv.Itoa(port))
	ftp.dataConn, ftp.Error = net.Dial("tcp", addr)
}

func (ftp *FTP) Login(user, passwd string) {
	ftp.Request("USER " + user)
	ftp.Request("PASS " + passwd)
	ftp.user = user
	ftp.passwd = passwd
}

func (ftp *FTP) Response() (code int, message string) {
	ret := make([]byte, 1024)
	n, _ := ftp.conn.Read(ret)
	msg := string(ret[:n])
	code, _ = strconv.Atoi(msg[:3])
	message = msg[4 : len(msg)-2]
	//	ftp.debugInfo("<*cmd*> " + ftp.cmd)
	//	ftp.debugInfo(fmt.Sprintf("<*code*> %d", code))
	//	ftp.debugInfo("<*message*> " + message)
	return
}

func (ftp *FTP) RetrResponse(filePath, fileName string) {
	ret := make([]byte, 4096)
	content := ""
	var sum int
	n, _ := ftp.dataConn.Read(ret)
	msg := string(ret[:n])
	sum = n
	content = content + msg
	for {
		if n > 0 {
			retr := make([]byte, 4096)
			n, _ = ftp.dataConn.Read(retr)
			sum = sum + n
			msg = string(retr[:n])
			content = content + msg
		} else {
			break
		}
	}
	//fmt.Println("content->", content)
	FtpWriteFile(filePath, fileName, content)
	fmt.Println("文件总大小->", sum)
	ftp.dataConn.Close()
}

/**
* ftp数据窗口的内容
 */
func (ftp *FTP) ListResponse() string {
	ret := make([]byte, 4096)
	content := ""
	var sum int
	n, _ := ftp.dataConn.Read(ret)
	msg := string(ret[:n])
	sum = n
	content = content + msg
	for {
		if n > 0 {
			retr := make([]byte, 4096)
			n, _ = ftp.dataConn.Read(retr)
			sum = sum + n
			msg = string(retr[:n])
			content = content + msg
		} else {
			break
		}
	}
	ftp.dataConn.Close()
	return content
}

/**
* 新建文件并写入内容
 */
func FtpWriteFile(filePath, fileName, content string) (int, error) {
	_, err := CreateFile(filePath)
	if err != nil {
		return 0, err
	}
	src := filePath + "/" + fileName
	fs, e := os.Create(src)
	if e != nil {
		return 0, e
	}
	defer fs.Close()
	return fs.WriteString(content)
}

/**
* 判断文件或者目录是否存在
 */
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

/**
* 创建文件
 */
func CreateFile(src string) (string, error) {
	//src := dir + name + "/"
	if IsExist(src) {
		return src, nil
	}

	if err := os.MkdirAll(src, 0777); err != nil {
		if os.IsPermission(err) {
			fmt.Println("你不够权限创建文件")
		}
		return "", err
	}

	return src, nil
}

func (ftp *FTP) Request(cmd string) {

	ftp.conn.Write([]byte(cmd + "\r\n"))
	ftp.cmd = cmd
	if cmd == "PASV" {
		ftp.Code, ftp.Message = ftp.Response()

		start := strings.Index(ftp.Message, "(")
		end := strings.LastIndex(ftp.Message, ")")

		// We have to split the response string
		pasvData := strings.Split(ftp.Message[start+1:end], ",")
		// Let's compute the port number
		portPart1, _ := strconv.Atoi(pasvData[4])

		portPart2, _ := strconv.Atoi(pasvData[5])

		// Recompose port
		ftp.pasv = portPart1*256 + portPart2
	} else if (cmd != "PASV") && (ftp.pasv > 0) {
		ftp.Code, ftp.Message = ftp.Response()
		ftp.Message = newRequest(ftp.host, ftp.pasv, ftp.stream)
		ftp.pasv = 0
		ftp.stream = nil
		ftp.Code, _ = ftp.Response()
	} else {
		ftp.Code, ftp.Message = ftp.Response()
	}
}

func (ftp *FTP) RetrRequest(filePath, fileName, cmd string) {
	ftp.conn.Write([]byte(cmd + "\r\n"))
	ftp.cmd = cmd
	ftp.RetrResponse(filePath, fileName)
}

/**
* 打开数据窗口请求
 */
func (ftp *FTP) ListRequest(cmd string) string {
	ftp.conn.Write([]byte(cmd + "\r\n"))
	ftp.cmd = cmd
	content := ftp.ListResponse()
	return content
}

func (ftp *FTP) Pasv() {
	ftp.Request("PASV")
}

func (ftp *FTP) Pwd() {
	ftp.Request("PWD")
}

func (ftp *FTP) Cwd(path string) {
	ftp.Request("CWD " + path)
}

func (ftp *FTP) Mkd(path string) {
	ftp.Request("MKD " + path)
}

func (ftp *FTP) Size(path string) (size int) {
	ftp.Request("SIZE " + path)
	size, _ = strconv.Atoi(ftp.Message)
	return
}

//(entries []*Entry, err error)
func (ftp *FTP) List(path string) []string {
	ftp.Pasv()
	por := ftp.pasv
	fileArr := []string{}
	ftp.openDataConn(por)
	if ftp.Error != nil {
		fmt.Println(ftp.Error)

	}
	content := ftp.ListRequest("LIST " + path)
	content = strings.Replace(content, "\r\n", "_", -1)
	content = strings.Replace(content, "            ", "+", -1)
	content = strings.Replace(content, "        ", "+", -1)
	content = strings.Replace(content, "    ", "+", -1)
	content = strings.Replace(content, "+   ", "+", -1)
	content = strings.Replace(content, "+", " ", -1)
	contentArr := strings.Split(content, "_")
	contLen := len(contentArr) - 1
	fileArr = contentArr[0:contLen]
	return fileArr
}

func (ftp *FTP) Stor(file string, data []byte) {
	ftp.Pasv()
	if data != nil {
		ftp.stream = data
	}
	ftp.Request("STOR " + file)
}

/**
* 从远程ftp上下载文件
* localPath 文件路径
* fileName 本地的文件名
* file FTP远程文件
 */
func (ftp *FTP) Retr(localPath, fileName, file string) {
	ftp.Request("REST 0")
	ftp.Pasv()
	por := ftp.pasv
	ftp.openDataConn(por)
	if ftp.Error != nil {
		fmt.Println(ftp.Error)
		ftp.dataConn.Close()
	}
	ftp.RetrRequest(localPath, fileName, "RETR "+file)

	//ftp.conn.Close()
}

func (ftp *FTP) Quit() {
	ftp.pasv = 0
	ftp.Request("QUIT")
	ftp.conn.Close()
}

// new connect to FTP pasv port, return data
func newRequest(host string, port int, b []byte) string {
	conn, _ := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	defer conn.Close()
	if b != nil {
		conn.Write(b)
		return "OK"
	}
	ret := make([]byte, 4096)
	n, _ := conn.Read(ret)
	return string(ret[:n])
}

