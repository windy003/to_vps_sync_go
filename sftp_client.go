package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPClient struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     *Config
}

func NewSFTPClient(config *Config) (*SFTPClient, error) {
	var authMethods []ssh.AuthMethod

	if config.SSH.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.SSH.Password))
	}

	if config.SSH.PrivateKeyPath != "" {
		key, err := os.ReadFile(config.SSH.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.SSH.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", config.SSH.Host, config.SSH.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %v", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("SFTP客户端创建失败: %v", err)
	}

	return &SFTPClient{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		config:     config,
	}, nil
}

func (c *SFTPClient) Close() error {
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.sshClient != nil {
		c.sshClient.Close()
	}
	return nil
}

func (c *SFTPClient) UploadFile(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer localFile.Close()

	remoteDir := filepath.Dir(remotePath)
	err = c.ensureRemoteDir(remoteDir)
	if err != nil {
		return fmt.Errorf("创建远程目录失败: %v", err)
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %v", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("文件传输失败: %v", err)
	}

	localStat, err := localFile.Stat()
	if err == nil {
		c.sftpClient.Chmod(remotePath, localStat.Mode())
		c.sftpClient.Chtimes(remotePath, localStat.ModTime(), localStat.ModTime())
	}

	return nil
}

func (c *SFTPClient) RemoveFile(remotePath string) error {
	return c.sftpClient.Remove(remotePath)
}

func (c *SFTPClient) RemoveDir(remotePath string) error {
	return c.sftpClient.RemoveDirectory(remotePath)
}

func (c *SFTPClient) ensureRemoteDir(remotePath string) error {
	remotePath = strings.ReplaceAll(remotePath, "\\", "/")
	
	_, err := c.sftpClient.Stat(remotePath)
	if err == nil {
		return nil
	}

	parent := filepath.Dir(remotePath)
	if parent != remotePath && parent != "/" && parent != "." {
		err = c.ensureRemoteDir(parent)
		if err != nil {
			return err
		}
	}

	return c.sftpClient.MkdirAll(remotePath)
}

func (c *SFTPClient) FileExists(remotePath string) bool {
	_, err := c.sftpClient.Stat(remotePath)
	return err == nil
}

func (c *SFTPClient) IsConnected() bool {
	if c.sshClient == nil {
		return false
	}
	
	_, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

func (c *SFTPClient) Reconnect() error {
	c.Close()
	
	newClient, err := NewSFTPClient(c.config)
	if err != nil {
		return err
	}
	
	c.sshClient = newClient.sshClient
	c.sftpClient = newClient.sftpClient
	return nil
}