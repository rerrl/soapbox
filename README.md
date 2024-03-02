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
