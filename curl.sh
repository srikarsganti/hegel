# curl -H "x-forwarded-for: 191.17.0.81" http://localhost:50061/v0/meta-data/ssh-public-keys
# curl -H "x-forwarded-for: 191.17.0.81" http://localhost:50061/v0/meta-data/disks/0
# curl -v -L -H "x-forwarded-for: 191.17.0.81" http://localhost:50061/v0/meta-data/00:50:56:9a:9e:d3/ipv6/0/netmask
# curl -v -H "x-forwarded-for: 191.17.0.81" http://localhost:50061/2009-04-04/meta-data/public-keys

# curl -v -H "x-forwarded-for: 191.17.0.81" -H 'Accept: application/json' http://localhost:50061/v0/meta-data

curl -v -H "x-forwarded-for: 191.17.0.81"  http://localhost:50061/v0/meta-data/00:50:56:9a:9e:d3/ipv4/0/ip

# curl -v -H "x-forwarded-for: 191.17.0.81"  http://localhost:50061/v0/user-data
