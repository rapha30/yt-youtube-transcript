# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads (the project has no third-party deps, so this is cheap).
COPY go.mod ./
RUN go mod download

COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/yt-transcript ./cmd/yt-transcript

# ---- runtime stage ----
# distroless/static ships CA certificates (needed for HTTPS to YouTube) and runs as
# a non-root user. No shell, no package manager: minimal attack surface.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/yt-transcript /usr/local/bin/yt-transcript
# /home/nonroot is owned by the nonroot user (65532), so the default `-out transcripts`
# is writable. A bare WORKDIR /work would be root-owned 0755 and unwritable for nonroot.
WORKDIR /home/nonroot
ENTRYPOINT ["yt-transcript"]
# To save files on the host, mount a writable volume and point -out at it:
#   docker run -v "$PWD/out:/data" IMAGE -out /data "https://youtu.be/VIDEO_ID"
CMD ["-h"]
