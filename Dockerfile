FROM mysql/mysql-server:8.0

COPY ./build/mysql_innodb_cluster_exporter /bin/mysql_innodb_cluster_exporter

EXPOSE 9105

ENTRYPOINT ["/bin/mysql_innodb_cluster_exporter"]
