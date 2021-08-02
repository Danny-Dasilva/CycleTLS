// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var proto_cycletls_pb = require('../proto/cycletls_pb.js');

function serialize_cyclestream_CycleTLSRequest(arg) {
  if (!(arg instanceof proto_cycletls_pb.CycleTLSRequest)) {
    throw new Error('Expected argument of type cyclestream.CycleTLSRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_cyclestream_CycleTLSRequest(buffer_arg) {
  return proto_cycletls_pb.CycleTLSRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_cyclestream_Response(arg) {
  if (!(arg instanceof proto_cycletls_pb.Response)) {
    throw new Error('Expected argument of type cyclestream.Response');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_cyclestream_Response(buffer_arg) {
  return proto_cycletls_pb.Response.deserializeBinary(new Uint8Array(buffer_arg));
}


// Interface exported by the server.
var CycleStreamService = exports.CycleStreamService = {
  // A Bidirectional streaming RPC.
//
// Accepts a stream of RouteNotes sent while a route is being traversed,
// while receiving other RouteNotes (e.g. from other users).
stream: {
    path: '/cyclestream.CycleStream/Stream',
    requestStream: true,
    responseStream: true,
    requestType: proto_cycletls_pb.CycleTLSRequest,
    responseType: proto_cycletls_pb.Response,
    requestSerialize: serialize_cyclestream_CycleTLSRequest,
    requestDeserialize: deserialize_cyclestream_CycleTLSRequest,
    responseSerialize: serialize_cyclestream_Response,
    responseDeserialize: deserialize_cyclestream_Response,
  },
};

exports.CycleStreamClient = grpc.makeGenericClientConstructor(CycleStreamService);
