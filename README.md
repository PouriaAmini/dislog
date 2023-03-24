# Dislog: Distributed Logging System

<img width="100" alt="Screenshot 2023-03-22 at 9 43 27 PM" src="https://user-images.githubusercontent.com/64161548/227077938-c08c20bf-6122-4b7a-948d-0998a7809ef7.png">

---

Dislog is a distributed logging system implemented in Go. It is designed to be scalable, fault-tolerant,
and easy to use. It allows you to collect and store logs from multiple sources in real-time.
Dislog is an open-source project and welcomes contributions from the community.

Visit out [Dislog Wiki] to learn about how Dislog is implemented.

---

## Running Dislog on Kubernetes

This is a guide on how to run Dislog implemented in Kubernetes locally using 
Kind. 
The system consists of several components managed by Helm that are deployed as 
Kubernetes resources using Helm charts.

### Prerequisites:

Before proceeding with the deployment, make sure that you have the following 
prerequisites:

- [Docker] installed on your local machine
- A running [Kubernetes] cluster on [Kind]
- [kubectl] command-line tool
- [Helm] installed on your local machine

### Installation

1. Clone or download the repository
    ```
    git clone https://github.com/pouriaamini/dislog
    ```
2. Create a Kind cluster on your local machine
    ```
    kind create cluster
    ```
3. Build the Docker image for dislog and load it into your Kind cluster
    ```
    make build-docker
    kind load docker-image github.com/pouriaamini/dislog:0.0.1
    ```
4. Install the Helm Chart for the system
    ```
    helm install dislog deploy/dislog
    ```
5. The current setup fires up three pods. You can list them by running 
`kubectl get pods`. When all three pods are ready, we can try requesting the API
    ```
    kubectl port-forward pod/dislog-0 8400 8400
    ```
   
### Verify the Setup
Run the command to request our service to get and print the list of servers
```
go run cmd/getservers/main.go
```
You should get the following output
```
servers:
- id:"dislog-0" rpc_addr:"dislog-0.dislog.default.svc.cluster.local:8400"
- id:"dislog-1" rpc_addr:"dislog-1.dislog.default.svc.cluster.local:8400"
- id:"dislog-2" rpc_addr:"dislog-2.dislog.default.svc.cluster.local:8400"
```

See our documentation on [GitHub Wiki](https://github.com/PouriaAmini/dislog/wiki/Deploy-Dislog-on-Google-Kubernetes-Engine) to run Dislog on the cloud.

[Docker]: https://docs.docker.com/engine
[Kubernetes]: https://kubernetes.io/
[Kind]: https://kubernetes.io/docs/tasks/tools/#kind
[kubectl]: https://kubernetes.io/docs/tasks/tools/#kubectl
[Helm]: https://helm.sh/docs/intro/install/
[Dislog Wiki]: https://github.com/PouriaAmini/dislog/wiki/Dislog-Implementation-Details
