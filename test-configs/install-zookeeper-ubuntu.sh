#!/usr/bin/env bash
set -e

echo "Installing Java..."
sudo apt-get -y --allow-unauthenticated install ansible

cat > /tmp/install-java.yml <<EOF
---
- name: a play that runs entirely on the ansible host
  hosts: localhost
  connection: local
  tasks:
  - name: Install Linux utils
    become: yes
    apt: name={{item}} state=latest
    with_items:
      - bash
      - curl
      - git
      - tar
      - iptables
      - iproute2
      - unzip

  - name: Install add-apt-repostory
    become: yes
    apt: name=software-properties-common state=latest

  - name: Add Oracle Java Repository
    become: yes
    apt_repository: repo='ppa:webupd8team/java'

  - name: Accept Java 8 License
    become: yes
    debconf: name='oracle-java8-installer' question='shared/accepted-oracle-license-v1-1' value='true' vtype='select'

  - name: Install Oracle Java 8
    become: yes
    apt: name={{item}} state=latest
    with_items:
      - oracle-java8-installer
      - ca-certificates
      - oracle-java8-set-default

  - name: Print Java version
    command: java -version
    register: result
  - debug:
      var: result.stderr

  - name: Print JDK version
    command: javac -version
    register: result
  - debug:
      var: result.stderr
EOF
ansible-playbook /tmp/install-java.yml

java -version
javac -version

echo "Installing Zookeeper..."
ZOOKEEPER_VERSION=3.4.9
sudo rm -rf $HOME/zookeeper
sudo curl -sf -o /tmp/zookeeper-$ZOOKEEPER_VERSION.tar.gz -L https://www.apache.org/dist/zookeeper/zookeeper-$ZOOKEEPER_VERSION/zookeeper-$ZOOKEEPER_VERSION.tar.gz
sudo tar -xzf /tmp/zookeeper-$ZOOKEEPER_VERSION.tar.gz -C /tmp/
sudo mv /tmp/zookeeper-$ZOOKEEPER_VERSION /tmp/zookeeper
sudo mv /tmp/zookeeper $HOME/
sudo chmod -R 777 $HOME/zookeeper/
mkdir -p $HOME/zookeeper/zookeeper.data
touch $HOME/zookeeper/zookeeper.data/myid
sudo chmod -R 777 $HOME/zookeeper/zookeeper.data/

cd $HOME/zookeeper
ls $HOME/zookeeper

echo "Done!"
