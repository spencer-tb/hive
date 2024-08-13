# Builds and runs the EEST (execution-spec-tests) consume engine simulator
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
#RUN git clone https://github.com/ethereum/execution-spec-tests.git --depth 1
RUN git clone https://github.com/spencer-tb/execution-spec-tests.git --depth 1 --branch consume-flag-tweaks
WORKDIR execution-spec-tests
RUN pip install --upgrade pip && \
    pip install -e .

## Define `consume engine` entry point using the local fixtures
ENTRYPOINT ["consume", "engine", "-v", "--stable"]
