FROM golang:1.23
RUN apt update
RUN apt install -y lsof netcat-traditional iptables
CMD ["/bin/bash"]


#FROM ubuntu:22.04
#RUN apt update && apt install -y wget bash coreutils lsof
#RUN yes | unminimize
#
#RUN wget https://golang.google.cn/dl/go1.23.4.linux-amd64.tar.gz
#RUN tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
#ENV PATH="$PATH:/usr/local/go/bin"
#
#
#RUN apt-get install -y ca-certificates openssl
#ARG cert_location=/usr/local/share/ca-certificates
## Get certificate from "github.com"
#RUN openssl s_client -showcerts -connect github.com:443 </dev/null 2>/dev/null|openssl x509 -outform PEM > ${cert_location}/github.crt
## Get certificate from "proxy.golang.org"
#RUN openssl s_client -showcerts -connect proxy.golang.org:443 </dev/null 2>/dev/null|openssl x509 -outform PEM >  ${cert_location}/proxy.golang.crt
## Update certificates
#RUN update-ca-certificates
#
#
#CMD [ "/bin/bash" ]