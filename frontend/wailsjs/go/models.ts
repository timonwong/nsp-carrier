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
	    validationErrors: host.ItemValidationError[];

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
	        this.validationErrors = this.convertValues(source["validationErrors"], host.ItemValidationError);
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
	    activeProfile: string;
	    profiles: host.Profile[];
	    validationErrors: host.ItemValidationError[];

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
	        this.activeProfile = source["activeProfile"];
	        this.profiles = this.convertValues(source["profiles"], host.Profile);
	        this.validationErrors = this.convertValues(source["validationErrors"], host.ItemValidationError);
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

export namespace host {

	export class ItemValidationError {
	    sourceId: string;
	    name: string;
	    code: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ItemValidationError(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceId = source["sourceId"];
	        this.name = source["name"];
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class Profile {
	    id: string;
	    displayName: string;
	    protocolFamily: string;
	    transport: string;
	    supportedExtensions: string[];
	    wireNamespace: string;
	    filesystemAccess: string;
	    compatibleVersions: string[];
	    verifiedVersions: string[];
	    knownIncompatibleVersions: string[];
	    adapterAvailable: boolean;

	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.protocolFamily = source["protocolFamily"];
	        this.transport = source["transport"];
	        this.supportedExtensions = source["supportedExtensions"];
	        this.wireNamespace = source["wireNamespace"];
	        this.filesystemAccess = source["filesystemAccess"];
	        this.compatibleVersions = source["compatibleVersions"];
	        this.verifiedVersions = source["verifiedVersions"];
	        this.knownIncompatibleVersions = source["knownIncompatibleVersions"];
	        this.adapterAvailable = source["adapterAvailable"];
	    }
	}

}

