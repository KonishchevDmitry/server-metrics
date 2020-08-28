# Prometheus metrics for my home server

Exposes the following metrics for each systemd unit:

```
# HELP server_metrics_errors Metrics collection errors.
# TYPE server_metrics_errors counter
server_metrics_errors 0

# HELP server_services_blkio_read_bytes Number of bytes read from the disk by the service.
# TYPE server_services_blkio_read_bytes gauge
server_services_blkio_read_bytes{device="md0",service="mongodb"} 3.6102144e+07

# HELP server_services_blkio_reads Number of read operations issued to the disk by the service.
# TYPE server_services_blkio_reads gauge
server_services_blkio_reads{device="md0",service="nginx"} 49

# HELP server_services_blkio_writes Number of write operations issued to the disk by the service.
# TYPE server_services_blkio_writes gauge
server_services_blkio_writes{device="md0",service="docker"} 11092

# HELP server_services_blkio_written_bytes Number of bytes written to the disk by the service.
# TYPE server_services_blkio_written_bytes gauge
server_services_blkio_written_bytes{device="md0",service="prometheus"} 2.121728e+06

# HELP server_services_cpu_system CPU time consumed in system (kernel) mode.
# TYPE server_services_cpu_system gauge
server_services_cpu_system{service="cron"} 0.48

# HELP server_services_cpu_user CPU time consumed in user mode.
# TYPE server_services_cpu_user gauge
server_services_cpu_user{service="ssh"} 1.41

# HELP server_services_memory_cache Page cache memory usage.
# TYPE server_services_memory_cache gauge
server_services_memory_cache{service="transmission"} 5.001216e+06

# HELP server_services_memory_rss Anonymous and swap cache memory usage.
# TYPE server_services_memory_rss gauge
server_services_memory_rss{service="plexmediaserver"} 8.787968e+07
```
