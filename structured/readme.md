notes

run go mod init sshdemo

add deps:

go get golang.org/x/crypto/ssh
go get github.com/pkg/sftp
go get github.com/hashicorp/vault/shamir


usage examples:

export SSH_HOST=<id@IP>
export SSH_PASS=<passcode>


```
task shamir SECRET="mysecret" N=5 K=3
task listkeys
task downloadkey FILE=key_01.json

go run ./main.go downloadkey -file "key_01.json" -dir "/tmp/keys" -out "key_01.json"

task automate
task monitor
task log MSG="This is a test log"
task exec CMD="hostname && whoami"


echo "test content" > myfile.txt
ls -l myfile.txt

task upload LOCAL=myfile.txt REMOTE=/tmp/myfile.txt

```