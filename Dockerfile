FROM golang:bookworm AS go

WORKDIR /moon
COPY . /moon
RUN apt-get update \
    && apt-get install --no-install-recommends -y libtesseract-dev
RUN go build -v -ldflags "-s -w -buildid=" ./cmd/moon


FROM python:3.11-slim AS py

RUN apt-get update \
    && apt-get install --no-install-recommends -y gcc libc6-dev
RUN mkdir /ffsubsync && pip install --target /ffsubsync ffsubsync


FROM ubuntu:24.04

RUN apt-get update \
    && apt-get install --no-install-recommends -y \
    libnss3 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    python3 \
    xz-utils \
    ffmpeg \
    libtesseract5 \
    tesseract-ocr-eng \
    && rm -rf /var/lib/apt/lists/*

ADD https://github.com/just-containers/s6-overlay/releases/latest/download/s6-overlay-noarch.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && rm /tmp/s6-overlay-noarch.tar.xz
ADD https://github.com/just-containers/s6-overlay/releases/latest/download/s6-overlay-x86_64.tar.xz /tmp
RUN tar -C / -Jxpf /tmp/s6-overlay-x86_64.tar.xz && rm /tmp/s6-overlay-x86_64.tar.xz

RUN mkdir -p /etc/services.d/moon \
    && printf '#!/command/with-contenv sh \n\
    mkdir -p /config/browser /root/.cache/rod \n\
    ln -sf /config/browser /root/.cache/rod/ \n\
    chown "${PUID}:${PGID}" /config /root /root/.cache/rod /config/browser \n\
    cd /config \n\
    exec s6-setuidgid "${PUID}:${PGID}" moon' > /etc/services.d/moon/run \
    && chmod +x /etc/services.d/moon/run

COPY --from=py /ffsubsync/ /ffsubsync/
RUN ln -s /usr/bin/python3 /usr/local/bin/python
ENV PYTHONPATH='/ffsubsync'
ENV PATH="/ffsubsync/bin:${PATH}"

COPY --from=go /moon/moon /usr/bin/

ENV TZ=Asia/Shanghai \
    PUID=1000 \
    PGID=1000
ENTRYPOINT ["/init"]