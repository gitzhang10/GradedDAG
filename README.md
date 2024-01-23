### 1. Install all dependencies
#### 1.1 Install go
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
#### 1.2 Install ansible
```
sudo apt update
sudo apt install software-properties-common
sudo apt-get install ansible
sudo apt-get install sshpass
```

### 2. Run the protocol
Download our code in your *WorkComputer* and build it.
#### 2.1 Generate configurations
You need to change the `config_gen/config_template.yaml` first, and next you can generate configurations for all *Servers*.
```
cd config_gen
go run main.go
```
#### 2.2 Run
Now you should enter the ansible directory to take the next operations.You need to change the `ansible/hosts` first.
##### 2.2.1 Login without passwords
The code below need some change
```
ansible -i ./hosts bit -m authorized_key -a "user=vagrant key='{{lookup('file', '/home/vagrant/.ssh/id_rsa.pub')}}' path='/home/vagrant/.ssh/authorized_keys' manage_dir=no" --ask-pass -c paramiko
```
##### 2.2.2 Configure servers via ansible
```
ansible-playbook conf-server.yaml -i hosts
```
##### 2.2.3 Run servers via ansible
```
ansible-playbook run-server.yaml -i hosts
```
##### 2.2.4 Kill servers via ansible
```
ansible-playbook clean-server.yaml -i hosts
```
   

 











