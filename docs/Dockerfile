FROM debian:bullseye-slim

RUN apt-get update && apt-get upgrade -y \
 && apt-get install -y \
    git \
    wget \
    git \
    curl \
    npm 

# RUN curl -sL https://deb.nodesource.com/setup_14.x -o nodesource_setup.sh
# RUN bash nodesource_setup.sh
# RUN apt-get install -y nodejs
RUN wget -c https://dl.google.com/go/go1.16.9.linux-amd64.tar.gz -O - | tar -xz -C /usr/local
RUN git clone https://github.com/Danny-Dasilva/CycleTLS.git
RUN export PATH=$PATH:/usr/local/go/bin
WORKDIR /

