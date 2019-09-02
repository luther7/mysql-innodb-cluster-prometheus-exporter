# MySQL InnoDB Cluster Prometheus Exporter

[travisci]: https://travis-ci.org/rubberydub/mysql-innodb-cluster-prometheus-exporter

A Prometheus exporter for MySQL InnoDB Cluster status

**This is not for production use - it's crude. I created this to learn the basics of using the Go Prometheus Client.**

This is a simple server that scrapes MySQL InnoDB Cluster status and exports them via HTTP for Prometheus consumption.

## To do

[ ] Testing a bad command is failing.
[ ] Add replica set size metric.

## Getting Started

To run it:

```bash
make run
```

Tests:
```bash
make test
```

Run with Docker:

```bash
make up
```

The Docker Compose uses an external Docker network. You can use [this Docker Compose](https://github.com/rubberydub/mysql-innodb-cluster-docker-compose) to run an InnoDB Cluster to check with the Exporter.

