#!/bin/sh
# Host metrics probe for Linux via /proc and df. Missing fields are omitted.
# CPU / net / disk-IO rates share one measured sample window (SAMPLE_MS via /proc/uptime).
export LC_ALL=C
printf 'HOSTNAME=%s\n' "$(hostname 2>/dev/null)"
read_uptime_ms() { awk '{printf "%d", ($1+0)*1000}' /proc/uptime 2>/dev/null; }
net_sample() {
  # Real NICs only: eth/ens/enp/eno/enx/wlan/wlp… with a backing /sys device.
  awk 'NR>2 {
    n=$1; sub(/:$/,"",n);
    if (n=="" || n !~ /^[a-zA-Z0-9._-]+$/) next;
    if (n !~ /^(eth|ens|enp|eno|enx|wlan|wlp|wlo|wlx)[0-9a-zA-Z]*$/) next;
    if (system("test -e /sys/class/net/" n "/device") != 0) next;
    printf "%s %s %s\n", n, $2, $10
  }' /proc/net/dev 2>/dev/null
}
disk_sample() {
  # Real HDD/SSD whole disks only; skip partitions, floppy, optical, loop/dm…
  # $6 sectors read, $10 sectors written; kernel sector = 512 bytes.
  awk '{
    n=$3;
    if (n=="" || n !~ /^(sd[a-z]+|hd[a-z]+|vd[a-z]+|xvd[a-z]+|nvme[0-9]+n[0-9]+|mmcblk[0-9]+)$/) next;
    if (system("test -e /sys/block/" n "/device") != 0) next;
    tpath = "/sys/block/" n "/device/type";
    if ((getline t < tpath) > 0) {
      close(tpath);
      if (t + 0 == 5) next;
    } else {
      close(tpath);
    }
    printf "%s %s %s\n", n, $6, $10
  }' /proc/diskstats 2>/dev/null
}
NET1=""; DISK1=""; NET2=""; DISK2=""
T0=$(read_uptime_ms)
t1=""; i1=""
if [ -r /proc/stat ]; then
  set -- $(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9, $5}' /proc/stat 2>/dev/null)
  t1=$1; i1=$2
fi
if [ -r /proc/net/dev ]; then
  NET1=$(net_sample)
fi
if [ -r /proc/diskstats ]; then
  DISK1=$(disk_sample)
fi
sleep 0.2
if [ -n "$t1" ]; then
  set -- $(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9, $5}' /proc/stat 2>/dev/null)
  t2=$1; i2=$2
  dt=$((t2-t1)); di=$((i2-i1))
  if [ -n "$dt" ] && [ "$dt" -gt 0 ] 2>/dev/null; then
    echo CPU_PCT=$(( (dt-di)*100/dt ))
  fi
fi
if [ -r /proc/diskstats ] && [ -n "$DISK1" ]; then
  DISK2=$(disk_sample)
fi
if [ -r /proc/net/dev ] && [ -n "$NET1" ]; then
  NET2=$(net_sample)
fi
T1=$(read_uptime_ms)
SAMPLE_MS=200
if [ -n "$T0" ] && [ -n "$T1" ]; then
  SAMPLE_MS=$((T1-T0))
fi
if [ -z "$SAMPLE_MS" ] || [ "$SAMPLE_MS" -lt 50 ] 2>/dev/null; then
  SAMPLE_MS=200
fi
echo SAMPLE_MS=$SAMPLE_MS
if [ -r /proc/meminfo ]; then
  awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{if(t>0){print "MEM_TOTAL_KB="t; print "MEM_AVAILABLE_KB="a}}' /proc/meminfo 2>/dev/null
fi
df -P -k / 2>/dev/null | awk 'NR==2 && $2+0>0 {print "DISK_TOTAL_KB="$2; print "DISK_USED_KB="$3}'
if [ -r /proc/loadavg ]; then
  awk '{print "LOAD1="$1}' /proc/loadavg 2>/dev/null
fi
if [ -n "$NET1" ] && [ -n "$NET2" ]; then
  printf '%s\n' "$NET1" | awk -v net2="$NET2" -v ms="$SAMPLE_MS" '
  BEGIN {
    n = split(net2, lines, "\n");
    for (i = 1; i <= n; i++) {
      if (lines[i] == "") continue;
      split(lines[i], a, " ");
      if (a[1] == "") continue;
      rx2[a[1]] = a[2] + 0;
      tx2[a[1]] = a[3] + 0;
    }
    dt = ms / 1000.0;
    if (dt <= 0) dt = 0.2;
  }
  {
    name = $1;
    if (name == "" || !(name in rx2)) next;
    rx = int((rx2[name] - ($2 + 0)) / dt);
    tx = int((tx2[name] - ($3 + 0)) / dt);
    if (rx < 0) rx = 0;
    if (tx < 0) tx = 0;
    printf "NET=%s:%d:%d\n", name, rx, tx;
  }'
fi
if [ -n "$DISK1" ] && [ -n "$DISK2" ]; then
  printf '%s\n' "$DISK1" | awk -v disk2="$DISK2" -v ms="$SAMPLE_MS" '
  BEGIN {
    n = split(disk2, lines, "\n");
    for (i = 1; i <= n; i++) {
      if (lines[i] == "") continue;
      split(lines[i], a, " ");
      if (a[1] == "") continue;
      rd2[a[1]] = a[2] + 0;
      wr2[a[1]] = a[3] + 0;
    }
    dt = ms / 1000.0;
    if (dt <= 0) dt = 0.2;
  }
  {
    name = $1;
    if (name == "" || !(name in rd2)) next;
    rd = int((rd2[name] - ($2 + 0)) * 512 / dt);
    wr = int((wr2[name] - ($3 + 0)) * 512 / dt);
    if (rd < 0) rd = 0;
    if (wr < 0) wr = 0;
    printf "DISKIO=%s:%d:%d\n", name, rd, wr;
  }'
fi
exit 0
