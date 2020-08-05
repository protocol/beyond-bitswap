import os
import json

from matplotlib import pyplot as plt

# def groupby(arr, key):
#     res = {}
#     for l in arr:
#         if !l[]
#         if len(res[""])
def process_result_line(l):
    l = json.loads(l)
    name = l["name"].split('/')
    value = (l["measures"])["value"]
    item = {}
    for attr in name:
        attr = attr.split(":")
        item[attr[0]] = attr[1]
    item["value"] = value
    return item

def aggregate_results():
    res = []
    for subdir, _, files in os.walk('./results'):
        for filename in files:
            filepath = subdir + os.sep + filename
            if filepath.split("/")[-1] == "results.out":
                print (filepath)
                resultFile = open(filepath, 'r')
                for l in resultFile.readlines():
                    res.append(process_result_line(l))
    return res

def groupBy(agg, metric):
    res = {}
    for item in agg:
        if not item[metric] in res:
            res[item[metric]] = []
        res[item[metric]].append(item)
    return res

def plot_latency(byLatency, byBandwidth, byFileSize):
    x = []
    y = {}
    print(len(byLatency), len(byBandwidth))

    p1, p2 = len(byLatency), len(byBandwidth)
    print(p1,p2)
    pindex = 1
    
    for l in byLatency:
        for b in byBandwidth:
            print(l,b)
            ax =plt.subplot(p1, p2, pindex)
            ax.set_title("latency: "+l + " bandwidth: " + b)

            for f in byFileSize:
                x.append(f)
                y[f] = []
                for i in byFileSize[f]:
                    if i["name"] == "time_to_fetch" and\
                        i["latencyMS"] == l and i["bandwidthMB"] == b and\
                            i["nodeType"]=="Leech":
                            y[f].append(i["value"])

                avg = []
                for i in y:
                    ax.scatter([i]*len(y[i]), y[i])
                    avg.append(sum(y[i])/len(y[i]))

                ax.plot(x, avg)
                pindex+=1
    plt.show()



if __name__ == "__main__":
    print("Starting to run...")
    agg = aggregate_results()
    byLatency = groupBy(agg, "latencyMS")
    byNodeType = groupBy(agg, "nodeType")
    byFileSize = groupBy(agg, "fileSize")
    byBandwidth = groupBy(agg, "bandwidthMB")

    plot_latency(byLatency, byBandwidth, byFileSize)
