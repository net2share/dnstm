# Bandwidth Benchmarks: Single vs Multi Mode (v0.5.0)

This document presents benchmark results for dnstm v0.5.0, comparing single-tunnel mode versus multi-tunnel mode (dnsrouter) for different tunnel types.

## Test Configuration

- **Test File:** 10MB (speedtest.tele2.net/10MB.zip)
- **Resolver:** 1.1.1.1:53 (Cloudflare)
- **Server:** v-slipstream (95.179.171.48)
- **Client:** Local machine via SOCKS5 proxy
- **Date:** 2026-01-29
- **Runs:** 5 rounds per configuration

### Modes Tested

| Mode       | Description                                                              |
| ---------- | ------------------------------------------------------------------------ |
| **Single** | Active transport binds directly to EXTERNAL_IP:53                        |
| **Multi**  | DNS router on EXTERNAL_IP:53 forwards to transport services on localhost |

## Results

### Slipstream SOCKS (slip1)

Uses DNS-over-QUIC protocol for tunneling with microsocks target.

| Mode   | Run 1  | Run 2  | Run 3  | Run 4  | Run 5  | **Average** |
| ------ | ------ | ------ | ------ | ------ | ------ | ----------- |
| Single | 66,795 | 65,866 | 57,663 | 63,777 | 61,742 | **63,169**  |
| Multi  | 57,500 | 68,843 | 59,464 | 62,992 | 57,769 | **61,314**  |

**Router Overhead: 3%**

### DNSTT SOCKS (dnstt1)

Uses DNS TXT record encoding for tunneling with microsocks target.

| Mode   | Run 1   | Run 2   | Run 3   | Run 4   | Run 5   | **Average** |
| ------ | ------- | ------- | ------- | ------- | ------- | ----------- |
| Single | 118,396 | 101,047 | 114,076 | 103,601 | 100,316 | **107,487** |
| Multi  | 99,063  | 93,307  | 106,566 | 97,794  | 101,549 | **99,656**  |

**Router Overhead: 7%**

### Slipstream + Shadowsocks (ss1)

Shadowsocks encryption over Slipstream DNS-over-QUIC tunnel.

| Mode   | Run 1  | Run 2  | Run 3  | Run 4  | Run 5  | **Average** |
| ------ | ------ | ------ | ------ | ------ | ------ | ----------- |
| Single | 57,733 | 65,916 | 59,157 | 57,572 | 65,323 | **61,140**  |
| Multi  | 57,267 | 65,194 | 59,825 | 57,598 | 64,818 | **60,940**  |

**Router Overhead: <1%**

## Summary

| Transport                  | Single Mode    | Multi Mode   | Overhead |
| -------------------------- | -------------- | ------------ | -------- |
| Slipstream SOCKS           | 63.2 KB/s      | 61.3 KB/s    | -3%      |
| DNSTT SOCKS                | **107.5 KB/s** | 99.7 KB/s    | -7%      |
| Slipstream + Shadowsocks   | 61.1 KB/s      | 60.9 KB/s    | <1%      |

### Key Observations

1. **DNSTT outperforms Slipstream** in these conditions (107 KB/s vs 63 KB/s)
2. **Multi-mode overhead is minimal** (1-7%) compared to previous benchmarks (26%)
3. **Shadowsocks encryption adds ~3% overhead** to Slipstream

### Protocol Comparison

| Protocol   | Single Mode | Notes                                        |
| ---------- | ----------- | -------------------------------------------- |
| DNSTT      | 107.5 KB/s  | Best performance, consistent                 |
| Slipstream | 63.2 KB/s   | QUIC protocol, higher variance               |
| SS over SL | 61.1 KB/s   | Encryption adds minimal overhead             |

DNSTT is approximately **1.7x faster** than Slipstream under current network conditions.

## Analysis

### Why Multi-Mode Overhead is Lower Now

In the new architecture, the DNS router is more efficient:
- Direct external IP binding (no NAT)
- Optimized packet forwarding
- No iptables rules in the data path

### Why DNSTT is Faster

Under these test conditions, DNSTT's TXT-record encoding proves more efficient than Slipstream's QUIC over DNS:
- DNSTT has optimized encoding for DNS payloads
- QUIC congestion control may be suboptimal over DNS
- Network path characteristics favor DNSTT

### Variance Analysis

| Transport  | Min     | Max     | Variance |
| ---------- | ------- | ------- | -------- |
| Slip-SOCKS | 57,500  | 68,843  | ±9%      |
| DNSTT-SOCKS| 93,307  | 118,396 | ±12%     |
| SS         | 57,267  | 65,916  | ±7%      |

## Recommendations

| Use Case                          | Recommended Transport           |
| --------------------------------- | ------------------------------- |
| Maximum bandwidth                 | **DNSTT SOCKS**                 |
| Encrypted traffic (Shadowsocks)   | **Slipstream + Shadowsocks**    |
| Multiple simultaneous tunnels     | **Multi-mode** (minimal overhead)|
| Simple single tunnel              | **Single mode** (any transport) |

### When to Use Each Mode

**Single Mode:**
- You only need one active tunnel at a time
- Slightly better performance (1-7%)
- Simpler debugging

**Multi Mode:**
- You need multiple tunnels simultaneously
- Domain-based routing required
- Performance impact is minimal

## Raw Data (bytes/sec)

### Slipstream SOCKS Single
```
Run 1: 66795
Run 2: 65866
Run 3: 57663
Run 4: 63777
Run 5: 61742
Avg:   63169
```

### Slipstream SOCKS Multi
```
Run 1: 57500
Run 2: 68843
Run 3: 59464
Run 4: 62992
Run 5: 57769
Avg:   61314
```

### DNSTT SOCKS Single
```
Run 1: 118396
Run 2: 101047
Run 3: 114076
Run 4: 103601
Run 5: 100316
Avg:   107487
```

### DNSTT SOCKS Multi
```
Run 1: 99063
Run 2: 93307
Run 3: 106566
Run 4: 97794
Run 5: 101549
Avg:   99656
```

### Shadowsocks Single
```
Run 1: 57733
Run 2: 65916
Run 3: 59157
Run 4: 57572
Run 5: 65323
Avg:   61140
```

### Shadowsocks Multi
```
Run 1: 57267
Run 2: 65194
Run 3: 59825
Run 4: 57598
Run 5: 64818
Avg:   60940
```

---

# Authoritative/Direct Mode Benchmarks

Testing with direct connection to server IP (bypassing recursive resolvers).

## Test Configuration (Authoritative)

- **Test File:** 10MB (speedtest.tele2.net/10MB.zip)
- **Connection:** Direct to server IP (95.179.171.48:53)
- **Server:** v-slipstream (95.179.171.48)
- **Client:** Local machine via SOCKS5 proxy
- **Date:** 2026-01-29
- **Runs:** 5 rounds per configuration

### Connection Modes

| Transport  | Client Flag                              |
| ---------- | ---------------------------------------- |
| Slipstream | `--authoritative 95.179.171.48:53`       |
| DNSTT      | `-udp 95.179.171.48:53`                  |
| Shadowsocks| plugin-opts: `authoritative=IP:53`       |

## Results (Authoritative Mode)

### Slipstream SOCKS (slip1)

| Mode   | Run 1     | Run 2     | Run 3     | Run 4     | Run 5     | **Average**   |
| ------ | --------- | --------- | --------- | --------- | --------- | ------------- |
| Single | 2,150,146 | 3,012,674 | 1,747,497 | 771,178   | 1,079,442 | **1,752,187** |
| Multi  | 1,253,103 | 1,880,321 | 3,029,553 | 3,759,885 | 2,803,593 | **2,545,291** |

**Router Overhead: -45%** (multi faster due to variance)

### DNSTT SOCKS (dnstt1)

| Mode   | Run 1     | Run 2     | Run 3     | Run 4     | Run 5     | **Average**   |
| ------ | --------- | --------- | --------- | --------- | --------- | ------------- |
| Single | 885,721   | 1,108,426 | 1,099,427 | 1,104,013 | 1,044,433 | **1,048,404** |
| Multi  | 1,089,687 | 1,109,616 | 1,119,963 | 1,120,441 | 1,061,680 | **1,100,277** |

**Router Overhead: -5%** (multi slightly faster)

### Slipstream + Shadowsocks (ss1)

| Mode   | Run 1     | Run 2     | Run 3     | Run 4   | Run 5   | **Average**   |
| ------ | --------- | --------- | --------- | ------- | ------- | ------------- |
| Single | 3,427,418 | 2,428,175 | 2,512,737 | 2,611,872 | 2,652,406 | **2,726,522** |
| Multi  | 2,431,489 | 3,347,296 | 2,963,229 | 670,975 | 401,200 | **1,962,838** |

**Router Overhead: 28%**

## Summary (Authoritative Mode)

| Transport                | Single Mode   | Multi Mode    | Overhead |
| ------------------------ | ------------- | ------------- | -------- |
| Slipstream SOCKS         | 1.75 MB/s     | **2.55 MB/s** | -45%     |
| DNSTT SOCKS              | 1.05 MB/s     | **1.10 MB/s** | -5%      |
| Slipstream + Shadowsocks | **2.73 MB/s** | 1.96 MB/s     | +28%     |

### Key Observations (Authoritative)

1. **Authoritative mode is ~25-45x faster** than recursive resolver mode
2. **Shadowsocks achieves the highest single-mode throughput** (2.73 MB/s)
3. **DNSTT is very consistent** with minimal variance between runs
4. **Slipstream shows high variance** (0.77-3.76 MB/s range)
5. **Multi-mode overhead varies by transport** (-45% to +28%)

### Protocol Comparison (Authoritative)

| Protocol   | Best Mode | Speed     | Notes                            |
| ---------- | --------- | --------- | -------------------------------- |
| SS over SL | Single    | 2.73 MB/s | Fastest, encryption adds benefit |
| Slipstream | Multi     | 2.55 MB/s | High variance, QUIC overhead     |
| DNSTT      | Multi     | 1.10 MB/s | Most consistent, reliable        |

### Variance Analysis (Authoritative)

| Transport  | Min       | Max       | Variance |
| ---------- | --------- | --------- | -------- |
| Slip-SOCKS | 771,178   | 3,759,885 | ±66%     |
| DNSTT-SOCKS| 885,721   | 1,120,441 | ±12%     |
| SS         | 401,200   | 3,427,418 | ±79%     |

## Comparison: Recursive vs Authoritative

| Transport  | Recursive | Authoritative | Speedup |
| ---------- | --------- | ------------- | ------- |
| Slip-SOCKS | 63 KB/s   | 1.75 MB/s     | **28x** |
| DNSTT-SOCKS| 107 KB/s  | 1.05 MB/s     | **10x** |
| SS         | 61 KB/s   | 2.73 MB/s     | **45x** |

### Why Authoritative Mode is Faster

1. **No resolver latency** - Direct UDP to server
2. **No rate limiting** - Recursive resolvers may throttle
3. **No caching issues** - Direct connection avoids resolver caching
4. **Lower RTT** - Single hop vs multiple hops

## Recommendations (Updated)

| Use Case                          | Recommended Setup                        |
| --------------------------------- | ---------------------------------------- |
| Maximum bandwidth                 | **SS + Authoritative + Single mode**     |
| Censorship bypass (stealth)       | **DNSTT + Recursive resolver**           |
| Reliable, consistent speed        | **DNSTT + Authoritative**                |
| Multiple domains needed           | **Multi-mode** (any transport)           |

### When to Use Each Connection Mode

**Authoritative Mode:**
- Direct connection to your server is possible
- Maximum bandwidth required
- Lower latency needed
- Less stealth required

**Recursive Resolver Mode:**
- Need to hide server IP
- Censorship environment
- Stealth is priority over speed

## Raw Data - Authoritative Mode (bytes/sec)

### Slipstream SOCKS Single (Authoritative)
```
Run 1: 2150146
Run 2: 3012674
Run 3: 1747497
Run 4: 771178
Run 5: 1079442
Avg:   1752187
```

### Slipstream SOCKS Multi (Authoritative)
```
Run 1: 1253103
Run 2: 1880321
Run 3: 3029553
Run 4: 3759885
Run 5: 2803593
Avg:   2545291
```

### DNSTT SOCKS Single (Direct)
```
Run 1: 885721
Run 2: 1108426
Run 3: 1099427
Run 4: 1104013
Run 5: 1044433
Avg:   1048404
```

### DNSTT SOCKS Multi (Direct)
```
Run 1: 1089687
Run 2: 1109616
Run 3: 1119963
Run 4: 1120441
Run 5: 1061680
Avg:   1100277
```

### Shadowsocks Single (Authoritative)
```
Run 1: 3427418
Run 2: 2428175
Run 3: 2512737
Run 4: 2611872
Run 5: 2652406
Avg:   2726522
```

### Shadowsocks Multi (Authoritative)
```
Run 1: 2431489
Run 2: 3347296
Run 3: 2963229
Run 4: 670975
Run 5: 401200
Avg:   1962838
```

## Notes

- All speeds in bytes/sec unless otherwise noted
- Authoritative/direct mode bypasses recursive resolvers
- High variance in Slipstream tests may be due to QUIC congestion control
- DNSTT shows most consistent performance across all tests
- Shadowsocks multi-mode had outlier runs (670K, 401K) affecting average
