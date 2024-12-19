# Yetis WIP <img src=".github/yetigopher.png" width="92" align="center" alt="i"/>
Kubernetes for linux processes running on a single machine.

### Use Case
Your VPS doesn't support Docker containers, but you would still like some `k8s` features.

## Features of Yetis
1. Kubernetes-like declarative configuration.
2. Self-healing. Automatically restarts failed processes. Kills and recreates unresponsive processes.
3. Log management. It saves the standard output into iterative log files.
4. Zero downtime deployment. Achieved with Service

## Installing
```shell
sudo wget -P /usr/local/bin https://github.com/glossd/yetis/raw/refs/heads/master/build/yetis && chmod +x /usr/local/bin/yetis 
```
## Commands
*You don't need to be `root`.
### Start Yetis server
```shell
yetis start
```
Yetis will start in the background. The logs are available at `/tmp/yetis.log`. You can specify your own log directory with `-d` flag.
### Available commands
#### Deploy your process:
```shell
yetis apply -f config.yaml
``` 
[Configuration](#full-configuration)  

#### List deployments
`yetis list` will show the list of the processes.    
Add flag `-w` to watch the updates
![](.github/yetis-list-w.gif)

#### Full list of commands
```
Server Commands:
	start [-d]              start Yetis server
	shutdown                terminate Yetis server
	info                    print server status
Resources Commands:
	apply -f FILENAME       apply a configuration from yaml file.
	get [-w] KIND           print a list the managed resources.
	logs [-f] NAME          print the logs of the deployment with NAME
	describe KIND NAME      Print a detailed description of the selected resource
	delete KIND NAME        delete the resource, terminating its process
	help                    print the list of the commands
```

## Configuration examples
### Deployment with Service
```yaml
kind: Deployment
spec:
  name: frontend
  cmd: npm start
  workdir: /home/user/myfront
  strategy: # WIP
    type: RollingUpdate
  env:
    - name: APP_PORT # If you configure a Service, your app shouldn't have a static port.
      value: $YETIS_PORT
---
kind: Service
spec:
  port: 2345
  selector:
    name: frontend
```
### Deployment without Service 
```yaml
kind: Deployment
spec:
  name: frontend
  cmd: npm start
  workdir: /home/user/myfront
  livenessProbe:
    tcpSocket:
      port: 3000
```

## Service configuration
Service is a proxy for the deployment with a static port. It allows RollingUpdate and zero deployment.  
```yaml
kind: Service
spec:
  port: 4567 # The port for Service to run on
  selector:
    name: name-of-deployment # Name of the deployment to proxy to.
```

## Deployment configuration
```yaml
kind: Deployment
spec:
  name: hello-world # Must be unique
  preCmd: javac HelloWorld.java # Command to execute before the starting the process.  
  cmd: java HelloWorld
  workdir: /home/user/myproject # Directory where command is executed. Defaults to the path in 'apply -f'. 
  logdir: /home/user/myproject/logs # Directory where the logs are stored. Defaults to the path in 'apply -f'.
  strategy: # WIP
    type: Recreate # Recreate or RollingUpdate. Defaults to Recreate. RollingUpdate should be specified only with Service 
  livenessProbe: # Checks if the command is alive and if not then restarts it
    tcpSocket:
      port: 8080 # Should be specified if Service is not configured. Defaults to $YETIS_PORT 
    initialDelaySeconds: 5 # Defaults to 10
    periodSeconds: 5 # Defaults to 10
    failureThreshold: 3 # Defaults to 3
    successThreshold: 1 # Defaults to 1
  env:
    - name: SOME_SECRET
      value: "pancakes are cakes made in a pan"
    - name: SOME_PASSWORD
      value: mellon
    - name: MY_PORT
      value: $YETIS_PORT # pass the value of the environment variable to another one.
```


### Liveness Probe
Checks if the process is alive and ready.  Yetis relies on this configuration to restart the process. Plus if Service is configured, then forward traffic to the Deployment. 
For now, the probe only supports tcpSocket. It also acts as Readiness and StartUp Probes. If you need anything, PRs are welcome.

### Deployment Strategies
Zero downtime is achieved with `RollingUpdate` strategy. It will spawn a new deployment, check if it's [healthy](#liveness-probe),
then direct traffic to the new instance, and only then will terminate the old instance.  
If you specify the `Recreate` strategy, Yetis will wait for the termination of the old instance before starting a new one.
It's the same as in [Kubernetes](https://medium.com/@muppedaanvesh/rolling-update-recreate-deployment-strategies-in-kubernetes-️-327b59f27202)
