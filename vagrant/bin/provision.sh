#!/usr/bin/env bash

sudo echo nameserver 8.8.8.8 >> /etc/resolv.conf
sudo yum remove git*
sudo yum clean all

sudo yum install -y epel-release


sudo yum install -y git

sudo yum install -y https://packages.endpointdev.com/rhel/7/main/x86_64/endpoint-repo.x86_64.rpm

wget http://rpms.remirepo.net/enterprise/remi-release-7.rpm
sudo rpm -Uvh remi-release-7*.rpm

sudo yum clean all

sudo yum install -y nginx
sudo yum install -y golang
sudo yum install -y supervisor
sudo yum install -y redis

rm -rf /etc/supervisord.conf
sudo ln -s /vagrant/conf/supervisord.conf /etc/supervisord.conf
sudo mkdir -p /etc/supervisor/conf.d/
sudo ln -s /vagrant/conf/supervisord/soapbox.conf /etc/supervisor/conf.d/soapbox.conf
sudo ln -s /vagrant/conf/supervisord/notifications.conf /etc/supervisor/conf.d/notifications.conf
sudo ln -s /vagrant/conf/supervisord/indexer.conf /etc/supervisor/conf.d/indexer.conf
sudo ln -s /vagrant/conf/supervisord/rooms.conf /etc/supervisor/conf.d/rooms.conf
sudo ln -s /vagrant/conf/supervisord/metadata.conf /etc/supervisor/conf.d/metadata.conf

echo 'export GOPATH="/home/vagrant/go"' >> /home/vagrant/.bashrc
echo 'export PATH="$PATH:${GOPATH//://bin:}/bin"' >> /home/vagrant/.bashrc
source /home/vagrant/.bashrc
mkdir -p $GOPATH/{bin,pkg,src}


sudo rpm -Uvh https://download.postgresql.org/pub/repos/yum/reporpms/EL-7-x86_64/pgdg-redhat-repo-latest.noarch.rpm

# add the repo for the yum postgres archive
sudo bash -c 'cat << EOF > /etc/yum.repos.d/pgdg-96.repo
[pgdg96-archive]
name=PostgreSQL 9.6 RPMs for RHEL/CentOS 7
baseurl=https://yum-archive.postgresql.org/9.6/redhat/rhel-7-x86_64
enabled=1
gpgcheck=1
gpgkey=https://yum.postgresql.org/keys/RPM-GPG-KEY-PGDG
EOF'

sudo yum install -y postgresql96-server postgresql96
sudo /usr/pgsql-9.6/bin/postgresql96-setup initdb

sudo systemctl start postgresql-9.6
sudo systemctl enable postgresql-9.6

sudo su - postgres -c "psql -a -w -f /var/www/db/database.sql"
sudo su - postgres -c "psql -t voicely -a -w -f /var/www/db/tables.sql"

sudo rm /var/lib/pgsql/9.6/data/pg_hba.conf
sudo ln -s /vagrant/conf/pg_hba.conf /var/lib/pgsql/9.6/data/pg_hba.conf

# TODO uncomment and get below working

# wget https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-7.8.1-x86_64.rpm
# wget https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-7.8.1-x86_64.rpm.sha512
# sudo rpm --install elasticsearch-7.8.1-x86_64.rpm

# sudo rm -rf /etc/nginx/nginx.conf
# sudo ln -s /vagrant/conf/nginx.conf /etc/nginx/nginx.conf

# mkdir -p $GOPATH/src/github.com/soapboxsocial/
# sudo ln -s /var/www/ $GOPATH/src/github.com/soapboxsocial/soapbox

# mkdir -p /conf/services
# sudo cp -p /var/www/conf/services/* /conf/services
# sudo chown nginx:nginx -R /conf/services

# sudo ln -s $GOPATH/src/github.com/soapboxsocial/soapbox/conf/services/ /conf/services

# sudo chown nginx:nginx -R /cdn/images
# sudo chmod -R 0777 /cdn/images

# sudo mkdir -p /cdn/stories/
# sudo chown nginx:nginx -R /cdn/stories
# sudo chmod -R 0777 /cdn/stories

# cd $GOPATH/src/github.com/soapboxsocial/soapbox && sudo go build -o /usr/local/bin/soapbox main.go
# cd $GOPATH/src/github.com/soapboxsocial/soapbox/cmd/indexer && sudo go build -o /usr/local/bin/indexer main.go
# cd $GOPATH/src/github.com/soapboxsocial/soapbox/cmd/rooms && sudo go build -o /usr/local/bin/rooms main.go
# cd $GOPATH/src/github.com/soapboxsocial/soapbox/cmd/stories && sudo go build -o /usr/local/bin/stories main.go

# crontab /vagrant/conf/crontab

touch /vagrant/provisioned

echo "Provisioning done! Run 'vagrant reload'"
