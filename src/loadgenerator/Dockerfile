FROM python:3.7.2-slim as base

FROM base as builder

RUN apt-get -qq update \
    && apt-get install -y --no-install-recommends \
        g++

COPY requirements.txt .

RUN python -m pip install --force-reinstall pip==20.1.1
RUN pip install --prefix=/install -r requirements.txt

FROM base
COPY --from=builder /install /usr/local

COPY . .
RUN chmod +x ./loadgen.sh
RUN apt-get -qq update \
    && apt-get install -y --no-install-recommends \
        curl libzmq3-dev
ENTRYPOINT ./loadgen.sh
