docker build -t testing_container .

docker run --name test testing_container
docker run -dit --name test testing_container

docker start test

docker exec -it test  /bin/bash 

docker run --name testing \
--rm -it --privileged -p 6006:6006 \
my_first_image

docker system prune -a