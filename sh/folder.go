package sh

import (
	"github.com/pkg/sftp"
	"github.com/wal-g/storages/storage"
	"github.com/wal-g/tracelog"
	"golang.org/x/crypto/ssh"
	"io"
	"fmt"
)

type Folder struct {
	client SftpClient
	path string
}

const (
	Port = "SSH_PORT"
	Password = "SSH_PASSWORD"
	Username = "SSH_USERNAME"
)

var SettingsList = []string{
	Port,
	Password,
	Username,
};

func NewFolderError(err error, format string, args ...interface{}) storage.Error {
	return storage.NewError(err, "SSH", format, args...)
}

func ConfigureFolder(prefix string, settings map[string]string) (storage.Folder, error) {
	host, path, err := storage.ParsePrefixAsURL(prefix)

	if err != nil {
		return nil, err
	}

	user := settings[Username]
	pass := settings[Password]
	port := settings[Port]

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprint(host, ":", port)
	sshClient, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, NewFolderError(err, "Fail connect via ssh. Address: %s", address)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, NewFolderError(err, "Fail connect via sftp. Address: %s", address)
	}

	return &Folder{
		extend(sftpClient), path,
	}, nil
}

// TODO close ssh and sftp connection
func closeConnection(client io.Closer)  {
	err := client.Close()
	if err != nil {
		tracelog.WarningLogger.FatalOnError(err)
	}
}

func (folder *Folder) GetPath() string {
	fmt.Println("get")
	return folder.path
}

func (folder *Folder) ListFolder() (objects []storage.Object, subFolders []storage.Folder, err error) {
	fmt.Println("list")
	client := folder.client
	path := folder.path

	filesInfo, err := client.ReadDir(folder.path)

	if err != nil {
		return nil, nil,
			NewFolderError(err, "Fail read folder '%s'", path)
	}

	for _, fileInfo := range filesInfo {
		if fileInfo.IsDir() {
			folder := &Folder{
				folder.client,
				client.Join(path, fileInfo.Name()),
			}
			subFolders = append(subFolders, folder)
		}

		object := storage.NewLocalObject(
			fileInfo.Name(),
			fileInfo.ModTime(),
		)
		objects = append(objects, object)
	}

	return
}

func (folder *Folder) DeleteObjects(objectRelativePaths []string) error {
	fmt.Println("delete")  
	client := folder.client

	for _, relativePath := range objectRelativePaths {
		path := client.Join(folder.path, relativePath)

		err := client.Remove(path)
		if err != nil {
			return NewFolderError(err, "Fail delete object '%s'", path)
		}
	}

	return nil
}

func (folder *Folder) Exists(objectRelativePath string) (bool, error)  {
	fmt.Println("exists")
	path := folder.client.Join()
	_, err := folder.client.Stat(path)

	if err != nil {
		return false, NewFolderError(
			err, "Fail check object existence '%s'", path,
		)
	}

	return true, nil
}

func (folder *Folder) GetSubFolder(subFolderRelativePath string) storage.Folder {
	fmt.Println("get sub")
	return &Folder{
		folder.client,
		folder.client.Join(folder.path, subFolderRelativePath),
	}
}

func (folder *Folder) ReadObject(objectRelativePath string) (io.ReadCloser, error) {
	fmt.Println("read");
	path := folder.client.Join(folder.path, objectRelativePath)
	file, err := folder.client.OpenFile(path)

	if err != nil {
		return nil, NewFolderError(err, "Fail open file '%s'", path)
	}

	return file, nil
}

func (folder *Folder) PutObject(name string, content io.Reader) error {
	fmt.Println("put");
	client := folder.client

	fmt.Println(name)

	err := client.Mkdir(folder.path)
	if err != nil {
		return NewFolderError(err, "Fail to create folder '%s'", folder.path)
	}

	filePath := client.Join(folder.path, name)
	file, err := client.CreateFile(filePath)
	if err != nil {
		return NewFolderError(err, "Fail to create file '%s'", filePath)
	}

	_, err = io.Copy(file, content)
	if err != nil {
		return NewFolderError(err, "Fail write content to file '%s'", filePath)
	}

	return nil
}