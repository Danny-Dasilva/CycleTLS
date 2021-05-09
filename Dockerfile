FROM ubuntu:18.04
RUN apt-get update
RUN apt-get install git npm curl
Run curl -sL https://deb.nodesource.com/setup_14.x -o nodesource_setup.sh
Run bash nodesource_setup.sh
Run apt-get install -y nodejs
RUN git clone https://github.com/Danny-Dasilva/CycleTLS.git

WORKDIR /home/node/app
