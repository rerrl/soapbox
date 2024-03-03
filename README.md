# Soapbox

## Running

To run the Soapbox server locally, it is recommended to use `vagrant`. This can be done by running the following in the `vagrant` directory.

```console
vagrant up
vagrant reload
```

The API is then available under the IP address: `192.168.33.16`

# ANDREW

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
