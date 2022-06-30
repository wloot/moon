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
    xvfb \
    python3 \
    python3-pip \
    python3-setuptools \
    python3-dev \
    python3-wheel \
    gcc \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install ffsubsync

COPY --from=go /moon/moon /usr/bin/

COPY --from=nevinee/s6-overlay:2.2.0.3-bin-is-softlink / /
RUN mkdir /etc/services.d/moon && printf \
    '#!/usr/bin/with-contenv bash\n exec s6-setuidgid \${PUID}:\${PGID} moon' \
    > /etc/services.d/moon/run && chmod +x /etc/services.d/moon/run

ENV TZ=Asia/Shanghai \
    PUID=1000 \
    PGID=1000
ENTRYPOINT ["/init"]
