export namespace app {
	
	export class LogEntry {
	    time: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	}
	export class QueueItem {
	    id: string;
	    name: string;
	    path: string;
	    size: number;
	    selected: boolean;
	    conflict: boolean;
	    status: string;
	    uniqueServedBytes: number;
	    wireBytes: number;
	    progress: number;
	    requested: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.selected = source["selected"];
	        this.conflict = source["conflict"];
	        this.status = source["status"];
	        this.uniqueServedBytes = source["uniqueServedBytes"];
	        this.wireBytes = source["wireBytes"];
	        this.progress = source["progress"];
	        this.requested = source["requested"];
	    }
	}
	export class ViewSnapshot {
	    state: string;
	    sessionId: string;
	    items: QueueItem[];
	    logs: LogEntry[];
	    selectedCount: number;
	    selectedBytes: number;
	    requestedBytes: number;
	    uniqueServedBytes: number;
	    wireBytes: number;
	    overallProgress: number;
	    canStart: boolean;
	    canStop: boolean;
	    hasConflict: boolean;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new ViewSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.sessionId = source["sessionId"];
	        this.items = this.convertValues(source["items"], QueueItem);
	        this.logs = this.convertValues(source["logs"], LogEntry);
	        this.selectedCount = source["selectedCount"];
	        this.selectedBytes = source["selectedBytes"];
	        this.requestedBytes = source["requestedBytes"];
	        this.uniqueServedBytes = source["uniqueServedBytes"];
	        this.wireBytes = source["wireBytes"];
	        this.overallProgress = source["overallProgress"];
	        this.canStart = source["canStart"];
	        this.canStop = source["canStop"];
	        this.hasConflict = source["hasConflict"];
	        this.lastError = source["lastError"];
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

