# Build the mattermost rotator
ARG DOCKER_BUILD_IMAGE=golang:1.15.8
ARG DOCKER_BASE_IMAGE=alpine:3.13

FROM ${DOCKER_BUILD_IMAGE} AS build
WORKDIR /rotator/
COPY . /rotator/
RUN apt-get update -yq && apt-get install -yq unzip
RUN make build


# Final Image
FROM ${DOCKER_BASE_IMAGE}
LABEL name="Mattermost Rotator" \
  maintainer="cloud-team@mattermost.com" \
  vendor="Mattermost" \
  distribution-scope="public" \
  architecture="x86_64" \
  url="https://mattermost.com" \
  io.k8s.description="Mattermost Rotator accelerates k8s cluster node rotations" \
  io.k8s.display-name="Mattermost Rotator"

ENV ROTATOR=/rotator/rotator \
    USER_UID=10001 \
    USER_NAME=rotator

RUN  apk update && apk add libc6-compat && apk add ca-certificates
COPY --from=build /rotator/build/_output/bin/rotator /rotator/rotator
COPY --from=build /rotator/build/bin /usr/local/bin

RUN  /usr/local/bin/user_setup
WORKDIR /rotator/

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
