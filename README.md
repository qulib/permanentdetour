# permanentdetour
Based on the Carleton University tool which redirects Sierra Web OPAC requests to Primo URLs, this is 
a tiny web service which redirects Voyager Web OPAC requests to Primo URLs.

```
Permanent Detour: A tiny web service which redirects Voyager Web OPAC requests to Primo URLs.
Usage: permanentdetour [flag...] [file...]
  -address string
        Address to bind on. (default ":8877")
  -primo string
        The subdomain of the target Primo instance, ?????.primo.exlibrisgroup.com. Defaults to "ocul-qu".
  -vid string
        VID parameter for Primo. Defaults to "01OCUL_QU:QU_DEFAULT".
  Environment variables read when flag is unset:
  PERMANENTDETOUR_ADDRESS
  PERMANENTDETOUR_PRIMO
  PERMANENTDETOUR_VID
```

The following redirects are supported (with examples in the Queen's context):

- Permalinks. `/vwebv/holdingsInfo?bibId=651520` is redirected to `https://ocul-qu.primo.exlibrisgroup.com/discovery/fulldisplay?docid=alma996515203405158&vid=01OCUL_QU:QU_DEFAULT`
- Patron login. `/patroninfo` is redirected to `https://ocul-crl.primo.exlibrisgroup.com/discovery/login?vid=01OCUL_CRL:CRL_DEFAULT`
- Author index, call number index, and title search index. For example, `/vwebv/search?searchArg=twain&searchCode=NAME` is redirected to `https://ocul-qu.primo.exlibrisgroup.com/discovery/browse?browseQuery=twain&browseScope=author&vid=01OCUL_QU:QU_DEFAULT`
- Searches. `/vwebv/search?searchArg=spiders&searchCode=GKEY^` redirects to `https://ocul-qu.primo.exlibrisgroup.com/discovery/search?query=title,contains,spiders&search_scope=MyInst_and_CI&tab=Everything&vid=01OCUL_QU:QU_DEFAULT`

Searches are not automatically translated to Primo syntax. To do so would require lexing and parsing Voyager searches, which is outside the immediate scope of this tool.
