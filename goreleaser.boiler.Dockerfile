FROM debian:stable-slim
RUN apt-get update -y \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        lib32gcc-s1 \
    && rm -rf /var/lib/apt/lists/*
RUN groupadd -r boiler
RUN useradd -r -g boiler -m -d /boiler -s /bin/bash boiler
USER boiler
WORKDIR /boiler
ENTRYPOINT ["boiler"]
COPY boiler /usr/local/bin
