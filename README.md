# jsbundletools

jsbundletools is a Go utility used to extract, repack and patch the `jsbundle` files used in React Native apps.  

# Usage:

### To extract a jsbundle file  
`jsbundletools -m unpack -p main.jsbundle -o output/`

### To repack a jsbundle file  
`jsbundletools -m pack -n patched.jsbundle -o output/`

### To patch a jsbundle file  
`jsbundletools -m patch -p main.jsbundle -n patched.jsbundle -d patches/`