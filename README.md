# homecode
Code I use at home to manage my servers and services

## Pre-requisites

My reference environement is https://github.com/desnoe/docker-shell

Just add sshpass on top of it:
```
sudo apt install sshpass
```

## Bootstrap the server
We assume a fresh minimal install of debian:

```
ansible-playbook -i envs/lab/inventory.ini bootstrap.yml -Kk
```

## Roles
