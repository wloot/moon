FROM golang AS go

WORKDIR /moon
COPY . /moon
RUN go build ./cmd/moon


FROM ubuntu:bionic

RUN apt-get update \
    && apt-get install --no-install-recommends -y \
    libnss3 \
    libxss1 \
    libasound2 \
    libxtst6 \
    libgtk-3-0 \
    libgbm1 \
    ca-certificates \
    fonts-liberation fonts-noto-color-emoji fonts-noto-cjk \
    tzdata \
    dumb-init \
    xvfb \
    python3 \
    python3-pip \
    python3-setuptools \
    python3-dev \
    python3-wheel \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install ffsubsync

COPY --from=go /moon/moon /usr/bin/

ENTRYPOINT ["dumb-init", "--"]
CMD moon
