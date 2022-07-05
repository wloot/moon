FROM golang AS go

WORKDIR /moon
COPY . /moon
RUN apt-get update -qq \
    && apt-get install -y -qq libtesseract-dev libleptonica-dev
RUN go build ./cmd/moon


FROM python as py

ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1
RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc
RUN python3 -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
RUN pip3 install ffsubsync


FROM ubuntu:20.04

RUN apt-get update \
    && apt-get install --no-install-recommends -y \
    libnss3 \
    libxss1 \
    libasound2 \
    libxtst6 \
    libgtk-3-0 \
    libgbm1 \
    ca-certificates \
    python3 \
    ffmpeg \
    xz-utils \
    libtesseract-dev \
    libleptonica-dev \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go /moon/moon /usr/bin/

COPY --from=py /opt/venv/ /opt/venv/
ENV PATH="/opt/venv/bin:$PATH"

ARG S6_OVERLAY_VERSION=3.1.0.1
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && rm /tmp/s6-overlay-noarch.tar.xz
ADD https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz && rm /tmp/s6-overlay-x86_64.tar.xz

ENV TZ=Asia/Shanghai \
    PUID=1000 \
    PGID=1000
ENTRYPOINT ["/init"]
RUN mkdir -p /etc/services.d/moon \
    && printf '#!/command/with-contenv sh \n\
    mkdir -p /config/browser/ /root/.cache/rod/ \n\
    chown -R "${PUID}:${PGID}" /root /config \n\
    ln -sf /config/browser /root/.cache/rod/ \n\
    cd /config \n\
    exec s6-setuidgid "${PUID}:${PGID}" moon' > /etc/services.d/moon/run \
    && chmod +x /etc/services.d/moon/run
