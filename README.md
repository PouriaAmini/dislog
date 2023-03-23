# Dislog: Distributed Logging System

<img width="100" alt="Screenshot 2023-03-22 at 8 08 28 PM" src="https://user-images.githubusercontent.com/64161548/227066648-e2c4a149-57b1-4927-88fe-e14c3690ba0b.png">

---

Dislog is a distributed logging system implemented in Go. It is designed to be scalable, fault-tolerant,
and easy to use. It allows you to collect and store logs from multiple sources in real-time.
Dislog is an open-source project and welcomes contributions from the community.

---

## To start using Dislog

See our documentation on [Dislog Docs].

If you want to build Dislog right away there are two options:

##### You have a working [Go environment].

```
mkdir -p $GOPATH/src/dislog
cd $GOPATH/src/dislog
git clone https://github.com/pouriaamini/dislog
cd dislog
make
```

##### You have a working [Docker environment].

```
git clone https://github.com/pouriaamini/dislog
cd dislog
make build-docker
```

[Docker environment]: https://docs.docker.com/engine
[Go environment]: https://go.dev/doc/install
[Dislog Docs]: https://
