FROM ubuntu:18.04
RUN apt-get update
RUN apt-get install  -y git npm curl wget
Run curl -sL https://deb.nodesource.com/setup_14.x -o nodesource_setup.sh
Run bash nodesource_setup.sh
Run apt-get install -y nodejs
Run wget -c https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz -O - | tar -xz -C /usr/local
RUN git clone https://github.com/Danny-Dasilva/CycleTLS.git
Run export PATH=$PATH:/usr/local/go/bin
WORKDIR /home/node/app
