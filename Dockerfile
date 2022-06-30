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
    ffmpeg \
    xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install ffsubsync

COPY --from=go /moon/moon /usr/bin/

ARG S6_OVERLAY_VERSION=3.1.0.1
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz

ENV TZ=Asia/Shanghai \
    PUID=1000 \
    PGID=1000
ENTRYPOINT ["/init"]
RUN mkdir -p /etc/services.d/moon && printf \
    '#!/command/with-contenv bash\nchown -R "${PUID}:${PGID}" /root\nexec s6-setuidgid "${PUID}:${PGID}" moon' \
    > /etc/services.d/moon/run && chmod +x /etc/services.d/moon/run
