export namespace api {
	
	export class PortInfo {
	    portPath: string;
	    portType: string;
	
	    static createFrom(source: any = {}) {
	        return new PortInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.portPath = source["portPath"];
	        this.portType = source["portType"];
	    }
	}
	export class Record {
	    index: number;
	    dir: string;
	    data: string;
	    size: number;
	    match?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Record(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.dir = source["dir"];
	        this.data = source["data"];
	        this.size = source["size"];
	        this.match = source["match"];
	    }
	}
	export class Stats {
	    tx: number;
	    rx: number;
	    matched: number;
	    diff: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tx = source["tx"];
	        this.rx = source["rx"];
	        this.matched = source["matched"];
	        this.diff = source["diff"];
	    }
	}
	export class State {
	    mode: string;
	    upperPort: string;
	    lowerPorts: PortInfo[];
	    baseline: Record[];
	    actual: Record[];
	    stats: Stats;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.upperPort = source["upperPort"];
	        this.lowerPorts = this.convertValues(source["lowerPorts"], PortInfo);
	        this.baseline = this.convertValues(source["baseline"], Record);
	        this.actual = this.convertValues(source["actual"], Record);
	        this.stats = this.convertValues(source["stats"], Stats);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

