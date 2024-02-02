# Bor fhEVM

This is a fork of [Bor](https://github.com/maticnetwork/bor) that integrates the fhEVM. Refer to the original README for more information.

### Deploying a Polygon DevNet with 

> [!NOTE]
> This was tested on Ubuntu 22.04.3 LTS on AWS

The following steps will deploy a full stack that runs a Polygon blockchain that support the fhEVM. The following containers will be deployed:
- 1 ganache container representing the root chain
- 3 heimdall containers (and 3 rabbitmq containers for communication AFAICT)
- 3 bor containers
- 1 kms container


Next we will detail steps to endup with the stack described above

##### Cloning Repositories

Clone `bor` (this repo) and `matic-cli` next to each other (this is important as all the scripts rely on this). You also need to checkout a specific commit for `matic-cli`
```bash
$ git clone https://github.com/zama-ai/bor
$ git clone https://github.com/maticnetwork/matic-cli
$ cd matic-cli && git checkout af71f6d55fd5bbfb301f88496c72a5bf8a394179
```

##### Install dependencies

This will install required dependencies
```bash
$ cd ../bor
# Working Directory: bor
$ sh 1-install_deps.sh
```

##### Generate keys

Generate keys under `bor/keys` (this will be copied into the bor docker image)
```bash
$ cd keys
# Working Directory: bor/keys
$ docker run -v $PWD:/usr/local/app ghcr.io/zama-ai/fhevm-tfhe-cli:v0.2.3 fhevm-tfhe-cli generate-keys -d .
```

Setup the `cks` into the kms and leave the others there.
```bash
$ cd ..
# Working Directory: bor
$ mv keys/cks kms-keys/cks.bin
```

##### Setup Node

This will setup the correct version of Node
```bash
# Working Directory: bor
$ sh 2-setup_node.sh
```

##### Bootstrap devnet

This will generate files for the devnet under `matic-cli/devnet/devnet`. At this point you can update the genesis file to make custom changes

```bash
# Working Directory: bor
$ sh 3-bootstrap_devnet.sh
```

##### Setup Devnet

The original `docker-compose` file (under matic-cli/devnet) doesn't include a kms container, so make sure to add a service named `kms`  (bor will try to connect to kms:50051 over the docker bridge network, so use the correct names). You should also change the path to the keys

```yaml
services:
  kms:
    image: "ghcr.io/zama-ai/kms:v0.1.3"
    container_name: kms
    networks:
      - devnet-network
    # Update with your own full path to bor/kms-keys
    volumes:
      - /home/ubuntu/bor/kms-keys/:/usr/src/kms-server/temp/:ro
```

Some values need to be changed in the genesis files of the 3 bor nodes `matic-cli/devnet/devnet/node<i>/bor/genesis.json`:
- set `period` to `7` seconds
- set `producerDelay` to `7` seconds
- init some accounts with funds by adding them under `"alloc"`: this is useful if you are willing to run fhevm tests later

```bash
# Working Directory: bor
$ cat <<< $(jq '.config.bor.period[]=7 | .config.bor.producerDelay[]=7' ../matic-cli/devnet/devnet/node0/bor/genesis.json) > ../matic-cli/devnet/devnet/node0/bor/genesis.json
$ cat <<< $(jq '.config.bor.period[]=7 | .config.bor.producerDelay[]=7' ../matic-cli/devnet/devnet/node1/bor/genesis.json) > ../matic-cli/devnet/devnet/node1/bor/genesis.json
$ cat <<< $(jq '.config.bor.period[]=7 | .config.bor.producerDelay[]=7' ../matic-cli/devnet/devnet/node2/bor/genesis.json) > ../matic-cli/devnet/devnet/node2/bor/genesis.json
```

##### Start devnet

This will start all the containers for the devnet.

```bash
# Working Directory: bor
$ sh 4-start_devnet.sh
```

##### Test deployment

You can use fhevm tests to make sure encrypted smart contracts are working

But if you want to test the whole Polygon stack, you can run the following script
```bash
# Working Directory: bor
$ sh 5-test_deployment.sh
```


### Cleaning

When done with the devnet, you can shut it down by following these steps:

- Remove the entire devnet directory that was created during bootstrap `sudo rm -rf matic-cli/devnet`
- Stopping all the containers `docker compose down` (from `matic-cli/devnet`)
- Delete bor and heimdall images
- Think about pruning containers, networks, and volumes