// package: cyclestream
// file: proto/cycletls.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as proto_cycletls_pb from "../proto/cycletls_pb";

interface ICycleStreamService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    stream: ICycleStreamService_IStream;
}

interface ICycleStreamService_IStream extends grpc.MethodDefinition<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response> {
    path: "/cyclestream.CycleStream/Stream";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<proto_cycletls_pb.CycleTLSRequest>;
    requestDeserialize: grpc.deserialize<proto_cycletls_pb.CycleTLSRequest>;
    responseSerialize: grpc.serialize<proto_cycletls_pb.Response>;
    responseDeserialize: grpc.deserialize<proto_cycletls_pb.Response>;
}

export const CycleStreamService: ICycleStreamService;

export interface ICycleStreamServer extends grpc.UntypedServiceImplementation {
    stream: grpc.handleBidiStreamingCall<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
}

export interface ICycleStreamClient {
    stream(): grpc.ClientDuplexStream<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
    stream(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
    stream(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
}

export class CycleStreamClient extends grpc.Client implements ICycleStreamClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public stream(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
    public stream(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<proto_cycletls_pb.CycleTLSRequest, proto_cycletls_pb.Response>;
}
