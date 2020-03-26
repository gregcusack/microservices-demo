FROM jenkins/jenkins:lts

# Scaling
RUN /usr/local/bin/install-plugins.sh kubernetes

# Metrics
RUN /usr/local/bin/install-plugins.sh metrics

# install Maven
USER root
RUN apt-get update && apt-get install -y maven

USER jenkins

EXPOSE 8080
EXPOSE 50000