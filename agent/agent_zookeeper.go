// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	zkTemplate = `tickTime={{.TickTime}}
dataDir={{.DataDir}}
clientPort={{.ClientPort}}
initLimit={{.InitLimit}}
syncLimit={{.SyncLimit}}
maxClientCnxns={{.MaxClientConnections}}
snapCount={{.SnapCount}}
{{range .Peers}}server.{{.MyID}}={{.IP}}:2888:3888
{{end}}
`
)

// ZookeeperConfig is zookeeper configuration.
// http://zookeeper.apache.org/doc/trunk/zookeeperAdmin.html
type ZookeeperConfig struct {
	TickTime             int64
	DataDir              string
	ClientPort           int64
	InitLimit            int64
	SyncLimit            int64
	MaxClientConnections int64
	SnapCount            int64
	Peers                []ZookeeperPeer
}

// ZookeeperPeer defines Zookeeper peer configuration.
type ZookeeperPeer struct {
	MyID int
	IP   string
}

var shell = os.Getenv("SHELL")

func init() {
	if len(shell) == 0 {
		shell = "sh"
	}
}

// startZookeeper starts Zookeeper.
func startZookeeper(fs *flags, t *transporterServer) error {
	if !exist(fs.javaExec) {
		return fmt.Errorf("Java binary %q does not exist", globalFlags.javaExec)
	}

	if err := os.RemoveAll(fs.zkDataDir); err != nil {
		return err
	}
	if err := os.MkdirAll(fs.zkDataDir, 0777); err != nil {
		return err
	}

	// Zookeeper requires correct relative-path for runtime
	// needs manual 'cd' into the Zookeeper working directory!
	if err := os.Chdir(fs.zkWorkDir); err != nil {
		return err
	}

	ipath := filepath.Join(fs.zkDataDir, "myid")
	plog.Infof("writing Zookeeper myid file %d to %s", t.req.ZookeeperConfig.MyID, ipath)
	if err := toFile(fmt.Sprintf("%d", t.req.ZookeeperConfig.MyID), ipath); err != nil {
		return err
	}

	peerIPs := strings.Split(t.req.PeerIPsString, "___")
	peers := []ZookeeperPeer{}
	for i := range peerIPs {
		peers = append(peers, ZookeeperPeer{MyID: i + 1, IP: peerIPs[i]})
	}
	cfg := ZookeeperConfig{
		TickTime:             t.req.ZookeeperConfig.TickTime,
		DataDir:              fs.zkDataDir,
		ClientPort:           t.req.ZookeeperConfig.ClientPort,
		InitLimit:            t.req.ZookeeperConfig.InitLimit,
		SyncLimit:            t.req.ZookeeperConfig.SyncLimit,
		MaxClientConnections: t.req.ZookeeperConfig.MaxClientConnections,
		Peers:                peers,
		SnapCount:            t.req.ZookeeperConfig.SnapCount,
	}
	tpl := template.Must(template.New("zkTemplate").Parse(zkTemplate))
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, cfg); err != nil {
		return err
	}
	zc := buf.String()

	plog.Infof("writing Zookeeper config file %q (config %q)", fs.zkConfig, zc)
	if err := toFile(zc, fs.zkConfig); err != nil {
		return err
	}

	// CHANGE THIS FOR DIFFERENT ZOOKEEPER RELEASE
	// https://zookeeper.apache.org/doc/trunk/zookeeperAdmin.html
	// THIS IS ONLY VALID FOR Zookeeper r3.4.9
	flagString := `-cp zookeeper-3.4.9.jar:lib/slf4j-api-1.6.1.jar:lib/slf4j-log4j12-1.6.1.jar:lib/log4j-1.2.16.jar:conf org.apache.zookeeper.server.quorum.QuorumPeerMain`

	args := []string{shell, "-c", fs.javaExec + " " + flagString + " " + fs.zkConfig}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = t.databaseLogFile
	cmd.Stderr = t.databaseLogFile
	cs := fmt.Sprintf("%s %s", cmd.Path, strings.Join(args[1:], " "))

	plog.Infof("starting database %q", cs)
	if err := cmd.Start(); err != nil {
		return err
	}
	t.cmd = cmd
	t.cmdWait = make(chan struct{})
	t.pid = int64(cmd.Process.Pid)
	plog.Infof("started database %q (PID: %d)", cs, t.pid)

	return nil
}
