FROM golang AS go

WORKDIR /rod
COPY . /rod
RUN go build ./cmd/moon

FROM ubuntu:bionic

RUN apt-get update && apt-get install --no-install-recommends -y \
    libnss3 \
    libxss1 \
    libasound2 \
    libxtst6 \
    libgtk-3-0 \
    libgbm1 \
    ca-certificates \
    fonts-liberation fonts-noto-color-emoji fonts-noto-cjk \
    tzdatadumb-init \
    xvfb &&
    rm -rf /var/lib/apt/lists/*

COPY --from=go /rod/rod-manager /usr/bin/

ENTRYPOINT ["dumb-init", "--"]
CMD rod-manager
