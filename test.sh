docker build -t my_first_image .

docker run --name test my_first_image

docker exec -it my_first_image

docker run --name testing \
--rm -it --privileged -p 6006:6006 \
my_first_image

docker run --name testing \
--rm -it --privileged -p 6006:6006 \
--mount type=bind,src=${DETECT_DIR},dst=/models/research/object_detection/images \
my_first_image
