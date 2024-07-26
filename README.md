## Description
This project is used to implement GradedDAG.You can learn more about GradedDAG in our *GradedDAG: An Asynchronous DAG-based BFT Consensus with Lower Latency* paper.

## Usage
### 1. Machine types
Machines are divided into two types:
- *WorkComputer*: just configure `servers` at the initial stage, particularly via `ansible` tool. 
- *Servers*: run daemons of `GradedDAG`, communicate with each other via P2P model.

### 2. Precondition
- Recommended OS releases: Ubuntu 18.04 (other releases may also be OK)
- Go version: 1.16+ (with Go module enabled)
- Python version: 3.6.9+

#### 2.1 Install go
```
sudo apt-get update
mkdir tmp
cd tmp
wget https://dl.google.com/go/go1.16.15.linux-amd64.tar.gz
sudo tar -xvf go1.16.15.linux-amd64.tar.gz
sudo mv go /usr/local

echo 'export PATH=$PATH:~/.local/bin:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go env -w GO111MODULE="on"  
go env -w GOPROXY=https://goproxy.io 
```
#### 2.2 Install ansible
```
sudo apt update
sudo apt install software-properties-common
sudo apt-get install ansible
sudo apt-get install sshpass
```

### 3. Run the protocol
Download our code in your *WorkComputer* and build it.
#### 3.1 Generate configurations
You need to change the `config_gen/config_template.yaml` first, and next you can generate configurations for all *Servers*.
```
cd config_gen
go run main.go
```
#### 3.2 Run
Now you should enter the ansible directory to take the next operations.You need to change the `ansible/hosts` first.
##### 3.2.1 Login without passwords
The following code needs to be adjusted according to your actual situation.
```
ansible -i ./hosts BFT -m authorized_key -a "user=root key='{{lookup('file', '/home/root/.ssh/id_rsa.pub')}}' path='/home/root/.ssh/authorized_keys' manage_dir=no" --ask-pass -c paramiko
```
##### 3.2.2 Configure servers via ansible
```
ansible-playbook copycode.yaml -i hosts
ansible-playbook copyconfig.yaml -i hosts
```
##### 3.2.3 Run servers via ansible
```
ansible-playbook run-server.yaml -i hosts
```
##### 3.2.4 Kill servers via ansible
```
ansible-playbook clean-server.yaml -i hosts
```
   

 











