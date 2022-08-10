#!/usr/bin/env bash
# https://www.digitalocean.com/community/tutorials/how-to-remove-docker-images-containers-and-volumes
echo ">>>>>>>>>>>>>>>>>SIMIFINII PLATFORM<<<<<<<<<<<<<<<<<<<<<"
echo "Cleanup options ... 1) Full 2) Partial"
read OPTION

function partialCleanup {
  echo "Stopping All docker Containers"
  docker stop $(docker ps -a -q)

  echo "Removing All Stopped docker Containers"
  docker rm $(docker ps -a -q)

  echo "Removing Exited Containers"
  docker ps -a
  docker rm $(docker ps -qa --no-trunc --filter "status=exited")

  echo "Removing Volumes"
  docker volume rm $(docker volume ls -qf dangling=true)
  docker volume ls -qf dangling=true | docker volume rm

  echo "Removing Networks"
  docker network ls
  docker network ls | grep "bridge"
  docker network ls | awk '$3 == "bridge" && $2 != "bridge" { print $1 }'
  docker network prune -f

  echo "Removing Dangling Images"
  docker images
  docker rmi $(docker images --filter "dangling=true" -q --no-trunc)
  docker images | grep "none"
  docker rmi $(docker images | grep "none" | awk '/ / { print $3 }')
}

function fullCleanup {
	echo "Performing Docker Cleanup"
	echo "Deleting all dangling images, networks not used by one container, all build caches, and stopped containers"
	docker system prune --all --force

	echo "Deleting all unused volumes"
	docker system prune --all --force --volumes
}

if [[ $OPTION == "Partial" ]]
then
  partialCleanup
else
  fullCleanup
fi

echo ">>>>>>>>>>>>>>>>>SIMIFINII PLATFORM<<<<<<<<<<<<<<<<<<<<<"
