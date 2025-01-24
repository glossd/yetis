# Yetis WIP <img src=".github/yetigopher.png" width="92" align="center" alt="i"/>
Kubernetes for linux processes running on a single machine.

### Use Case
Your VPS doesn't support Docker containers, but you would still like some `k8s` features.

## Features of Yetis
1. Kubernetes-like declarative configuration.
2. Self-healing. Automatically restarts failed processes. Kills and recreates unresponsive processes.
3. Log management. It saves the standard output into iterative log files.
4. Zero downtime deployment. Achieved through `Service` and `RollingUpdate` strategy.

## Installing
```shell
sudo wget -O /usr/local/bin/yetis https://github.com/glossd/yetis/raw/refs/heads/master/build/yetis && chmod +x /usr/local/bin/yetis 
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
`yetis get` will show the list of the processes.    
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
	describe KIND NAME      print a detailed description of the selected resource
	delete KIND NAME        delete the resource, terminating its process
	restart NAME            restart the deployment according to its strategy type 
	help                    print the list of the commands
```

## Configuration examples
### Deployment
A simple process to watch over and restart if port becomes unavailable
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
### Deployment with Zero downtime
Service will on static port 2345 and proxy all requests to `frontend` Deployment which must have a dynamic port.
```yaml
kind: Deployment
spec:
  name: frontend
  cmd: npm start
  workdir: /home/user/myfront
  strategy:
    type: RollingUpdate # For zero downtime
  env:
    - name: APP_PORT # When you specify proxy, your app shouldn't have a static port.
      value: $YETIS_PORT
  proxy:
    port: 2345 # Forwards traffic from 2345 to the port in YETIS_PORT env var. 
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
  strategy:
    type: Recreate # Recreate or RollingUpdate. Defaults to Recreate.
  livenessProbe: # Checks if the command is alive and if not then restarts it
    tcpSocket:
      port: 8080 # Should be specified if proxy is not configured. Defaults to $YETIS_PORT 
    initialDelaySeconds: 5 # Defaults to 10
    periodSeconds: 5 # Defaults to 10
    failureThreshold: 3 # Defaults to 3
    successThreshold: 1 # Defaults to 1
  env: # YETIS_PORT env var is passed by default. You should use alongside proxy config. 
    - name: SOME_SECRET
      value: "pancakes are cakes made in a pan"
    - name: SOME_PASSWORD
      value: mellon
    - name: MY_PORT
      value: $YETIS_PORT # pass the value of the environment variable to another one.
  proxy:
    port: 8080 # Tells linux to forward from the specified port to $YETIS_PORT, allowing zero downtime restarts.
```

### Liveness Probe
Checks if the process is alive and ready.  Yetis relies on this configuration to restart the process. Plus if Service is configured, then forward traffic to the Deployment. 
For now, the probe only supports tcpSocket. It also acts as Readiness and StartUp Probes. If you need anything, PRs are welcome.

### Deployment Strategies
Zero downtime is achieved with `RollingUpdate` strategy and `restart` command. Your deployment must start on `$YETIS_PORT` and have a `Service` pointing at it. `restart` command will spawn a new deployment, check if it's [healthy](#liveness-probe),
then direct traffic to the new instance, and only then will terminate the old instance. The new deployment will have the name with an index i.e. frontend-1, frontend-2 and so on.
If deployment has `Recreate` strategy, Yetis will wait for the termination of the old instance before starting a new one with the same name.
It's the same as in [Kubernetes](https://medium.com/@muppedaanvesh/rolling-update-recreate-deployment-strategies-in-kubernetes-️-327b59f27202)
