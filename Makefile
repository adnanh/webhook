DOCKER_IMAGE_NAME=adnanh/webhook
CONTAINER_NAME=webhook

docker-build: Dockerfile
	docker build --force-rm=true --tag=${DOCKER_IMAGE_NAME} .

docker-run:
	@echo "Here's an example command on how to run a webhook container:"
	@echo "docker run -d -p 9000:9000 -v /etc/webhook:/etc/webhook --name=${CONTAINER_NAME} \\"
	@echo "  ${DOCKER_IMAGE_NAME} -verbose -hooks=/etc/webhook/hooks.json -hotreload"
