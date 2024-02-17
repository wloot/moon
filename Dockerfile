FROM golang:alpine AS go

WORKDIR /moon
COPY . /moon
RUN apk --no-cache add tesseract-ocr-dev
RUN go build -v -ldflags "-s -w -buildid=" ./cmd/moon


FROM python:3.11-alpine AS py

RUN apk --no-cache add gcc musl-dev
RUN mkdir /ffsubsync && pip install --target /ffsubsync ffsubsync


FROM alpine:latest

RUN apk --no-cache add \
    python3 \
    xz \
    ffmpeg \
    tesseract-ocr \
    tesseract-ocr-data-eng

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
ENV PYTHONPATH='/ffsubsync'
ENV PATH="/ffsubsync/bin:${PATH}"

COPY --from=go /moon/moon /usr/bin/

ENV TZ=Asia/Shanghai \
    PUID=1000 \
    PGID=1000
ENTRYPOINT ["/init"]
