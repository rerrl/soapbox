# Soapbox

## Running

To run the Soapbox server locally, it is recommended to use `vagrant`. This can be done by running the following in the `vagrant` directory.

```console
vagrant up
vagrant reload
```

The API is then available under the IP address: `192.168.33.16`

# Development

how to get this thing going (linux)

- have virtualbox installed
- install vagrant https://developer.hashicorp.com/vagrant/docs/installation
- install vbox guest plugin for vagrant: `vagrant plugin install vagrant-vbguest`
- try `vagrant up` command as defined above
- if this does not work becuase of address range issue, do the following:
- create file: `touch /etc/vbox/networks.conf` and add the desired address range by making this new file look like:

```
* 192.168.33.0/24
```

- try `vagrant up` again
- after this completes, run `vagrant reload`

## tips:

- destroy vm: `vagrant destroy -f`
- halt and vagrant up: `vagrant reload`
- ssh into vm: `vagrant ssh`
- sometimes on destroy, you may need to remove the `vagrant/provisioned` file to get a fresh start

if you want to test locally without vagrant, from the root of the project, run `go run main.go -c conf/services/soapbox.toml`. As of right now things will be broken becuase no services are configured locally. Maybe docker-compose this to get it working locally?

rebuild script:

```
#!/bin/bash

export GOPATH=/home/vagrant/go

# copy latest configuration files
sudo cp -r $GOPATH/src/github.com/soapboxsocial/soapbox/conf/services/* /conf/services/

# Change directory and build the project
echo "building soapbox main.go..."
cd $GOPATH/src/github.com/soapboxsocial/soapbox
sudo go build -o /usr/local/bin/soapbox main.go

# Change to rooms directory to build rooms
echo "building rooms main.go..."
cd $GOPATH/src/github.com/soapboxsocial/soapbox/cmd/rooms
sudo go build -o /usr/local/bin/rooms main.go

# Find the process ID of rooms server
pid=$(ps aux | grep "/usr/local/bin/rooms server" | grep -v grep | awk '{print $2}')


# If a PID exists, kill the process
if [ ! -z "$pid" ]; then
    echo "Killing rooms process with PID: $pid"
    sudo kill $pid
    echo "Killed rooms process."
else
    echo "No rooms process found."
fi

# Find the process ID of soapbox application, excluding the grep process itself
pid=$(ps aux | grep "/usr/local/bin/soapbox" | grep -v grep | awk '{print $2}')

# If a PID exists, kill the process
if [ ! -z "$pid" ]; then
    echo "Killing soapbox process with PID: $pid"
    sudo kill $pid
    echo "Killed soapbox process."
else
    echo "No soapbox process found."
fi

echo "Done."

```
