# Builds and runs the EEST (execution-spec-tests) consume engine simulator
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/ethereum/execution-spec-tests.git --depth 1
WORKDIR execution-spec-tests
RUN pip install --upgrade pip
RUN pip install -e .

## Define `consume engine` entry point with latest fixture release
ENTRYPOINT ["consume", "engine", "--input=https://github.com/ethereum/execution-spec-tests/releases/latest/download/fixtures_develop.tar.gz", "-v", "--fork", "Prague"]
