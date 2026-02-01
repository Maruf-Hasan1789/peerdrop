export namespace app {
	
	export class Settings {
	    user_name: string;
	    download_path: string;
	    is_permission_required_to_send_files: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_name = source["user_name"];
	        this.download_path = source["download_path"];
	        this.is_permission_required_to_send_files = source["is_permission_required_to_send_files"];
	    }
	}
	export class transferHistory {
	    peer: string;
	    file_name: string;
	    transfer_type: string;
	    status: string;
	    time_stamp: number;
	
	    static createFrom(source: any = {}) {
	        return new transferHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peer = source["peer"];
	        this.file_name = source["file_name"];
	        this.transfer_type = source["transfer_type"];
	        this.status = source["status"];
	        this.time_stamp = source["time_stamp"];
	    }
	}

}

export namespace discovery {
	
	export class Peer {
	    ID: string;
	    Name: string;
	    Addresses: number[][];
	    Port: number;
	    Version: string;
	    UserName: string;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Addresses = source["Addresses"];
	        this.Port = source["Port"];
	        this.Version = source["Version"];
	        this.UserName = source["UserName"];
	    }
	}
	export class PeerDTO {
	    id: string;
	    name: string;
	    addresses: string[];
	    port: number;
	    version: string;
	    user_name: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.addresses = source["addresses"];
	        this.port = source["port"];
	        this.version = source["version"];
	        this.user_name = source["user_name"];
	    }
	}

}

export namespace session {
	
	export class PeerSession {
	
	
	    static createFrom(source: any = {}) {
	        return new PeerSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

