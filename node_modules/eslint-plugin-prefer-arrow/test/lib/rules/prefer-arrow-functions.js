/**
 * @fileoverview Tests for prefer-arrow-functions rule.
 * @author Triston Jones
 */

'use strict';

const singleReturnOnly = (code, extraRuleOptions) => ({
  code,
  options: [Object.assign({singleReturnOnly: true}, extraRuleOptions)],
  parserOptions: {sourceType: 'module'}
});

var rule = require('../../../lib/rules/prefer-arrow-functions'),
    RuleTester = require('eslint').RuleTester;

var tester = new RuleTester({parserOptions: {ecmaVersion: 6}});
tester.run('lib/rules/prefer-arrow-functions', rule, {
  parserOptions: {ecmaVersion: 6},
  valid: [
    'var foo = (bar) => bar;',
    'var foo = bar => bar;',
    'var foo = bar => { return bar; }',
    'var foo = () => 1;',
    'var foo = (bar, fuzz) => bar + fuzz',
    '["Hello", "World"].reduce((p, a) => p + " " + a);',
    'var foo = (...args) => args',
    'class obj {constructor(foo){this.foo = foo;}}; obj.prototype.func = function() {};',
    'class obj {constructor(foo){this.foo = foo;}}; obj.prototype = {func: function() {}};',
    'var foo = function() { return this.bar; };',
    'function * testGenerator() { return yield 1; }',
    'const foo = { get bar() { return "test"; } }',
    'const foo = { set bar(xyz) {} }',
    'class foo { get bar() { return "test"; } }',
    'class foo { set bar(xyz) { } }',
    'class foo { bar() { return "test" } }',
    'function foo() { return () => this; }',
    'function foo() { return (bar = this) => bar; }',
    'function foo(bar = this) { return bar; }',
    'const foo = function () { return () => this; }',
    'const foo = function () { return (bar = this) => bar; }',
    'const foo = function (bar = this) { return bar; }',
    ...[
      'var foo = (bar) => {return bar();}',
      'function foo(bar) {bar()}',
      'var x = function foo(bar) {bar()}',
      'var x = function(bar) {bar()}',
      'function foo(bar) {/* yo */ bar()}',
      'function foo() {}',
      'function foo(bar) {bar(); return bar()}',
      'class MyClass { foo(bar) {bar(); return bar()} }',
      'var MyClass = { foo(bar) {bar(); return bar()} }',
      'export default function xyz() { return 3; }',
      'class MyClass { render(a, b) { return 3; } }'
    ].map(singleReturnOnly),
    ...[
      'const foo = async bar => bar;',
      'const foo = async bar => await Promise.resolve(2);',
      'const foo = async (a, b) => { return await Promise.resolve(2); }',
      'class MyClass { async foo(bar) { return bar; } }',
    ].map(code => ({ code, parserOptions: { ecmaVersion: 2017 } })),

    // Valid tests for "allowStandaloneDeclarations" option
    {code: 'function foo() { return "bar"; }', options: [{ allowStandaloneDeclarations: true }]},
    {code: 'function * fooGen() { return yield "bar"; }', options: [{ allowStandaloneDeclarations: true }]},
    {code: 'async function foo() { return await "bar"; }', options: [{ allowStandaloneDeclarations: true }], parserOptions: { ecmaVersion: 2017 }},
    {code: 'function foo() { return () => "bar"; }', options: [{ allowStandaloneDeclarations: true }]},
    {code: 'module.exports = function() { return "bar"; }',  options: [{ allowStandaloneDeclarations: true }]},
    {code: 'module.exports.foo = function() { return "bar"; }',  options: [{ allowStandaloneDeclarations: true }]},
    {code: 'exports.foo = function() { return "bar"; }',  options: [{ allowStandaloneDeclarations: true }]},
    {
      code: 'export function foo() { return "bar"; }',
      options: [{ allowStandaloneDeclarations: true }],
      parserOptions: { sourceType: 'module'},
    },
    {
      code: 'export default function() { return "bar"; }',
      options: [{ allowStandaloneDeclarations: true }],
      parserOptions: { sourceType: 'module'},
    },
    {
      code: 'export default function foo() { return "bar"; }',
      options: [{ allowStandaloneDeclarations: true }],
      parserOptions: { sourceType: 'module'},
    },
    {
      // Make sure "allowStandaloneDeclarations" works with typescript
      code: 'function foo(a: string): string { return `bar ${a}`;}',
      options: [{ allowStandaloneDeclarations: true }],
      parser: require.resolve('@typescript-eslint/parser')
    },

    {
      code: 'class MyClass { constructor() { this.x = 0; } add = (y) => { this.x += y; }; }',
      options: [{ classPropertiesAllowed: true }],
      parser: require.resolve('babel-eslint')
    }
  ],
  invalid: [
    {code: 'function foo() { return "Hello!"; }', errors: ['Use const or class constructors instead of named functions']},
    {code: 'function foo() { return arguments; }', errors: ['Use const or class constructors instead of named functions']},
    {code: 'function foo() { return function () { return this; }; }', errors: ['Use const or class constructors instead of named functions']},
    {code: 'function foo() { return function (bar = this) { return bar; }; }', errors: ['Use const or class constructors instead of named functions']},
    {code: 'const foo = function () { return function (bar = this) { return bar; }; }', errors: ['Prefer using arrow functions over plain functions']},
    {code: 'const foo = function () { return function () { return this; }; }', errors: ['Prefer using arrow functions over plain functions']},
    {code: 'var foo = function() { return "World"; }', errors: ['Prefer using arrow functions over plain functions']},
    {code: '["Hello", "World"].reduce(function(a, b) { return a + " " + b; })', errors: ['Prefer using arrow functions over plain functions']},
    {code: 'class obj {constructor(foo){this.foo = foo;}}; obj.prototype.func = function() {};', errors: ['Prefer using arrow functions over plain functions'], options: [{disallowPrototype:true}]},

    // Invalid tests for "allowStandaloneDeclarations" option
    {code: 'var foo = function() { return "bar"; }', errors: ['Prefer using arrow functions over plain functions'], options: [{ allowStandaloneDeclarations: true }]},
    {code: 'class FooClass { foo() { return "bar" }}', errors: ['Prefer using arrow functions over plain functions'], options: [{ allowStandaloneDeclarations: true, classPropertiesAllowed: true }]},
    {code: 'exports = function() { return "bar"; }',  options: [{ allowStandaloneDeclarations: true }], errors: ['Prefer using arrow functions over plain functions']},
    {
      // We are using multiple lines to check that it only errors on the inner function
      code: `function top() {
        return function inner() { return "bar"; };
      }`,
      errors: [{ message: 'Prefer using arrow functions over plain functions', line: 2 }],
      options: [{ allowStandaloneDeclarations: true }]
    },
    {
      // Make sure "allowStandaloneDeclarations" works with typescript
      code: `function foo(a: string): () => string {
        return function() { return \`bar \${a}\`; };
      }`,
      errors: [{ message: 'Prefer using arrow functions over plain functions', line: 2}],
      options: [{ allowStandaloneDeclarations: true }],
      parser: require.resolve('@typescript-eslint/parser')
    },
    
    ...[
      // Make sure it works with ES6 classes & functions declared in object literals (Babel only)
      [
        'class MyClass { render(a, b) { return 3; } }',
        'class MyClass { render = (a, b) => 3; }',
        { classPropertiesAllowed: true },
        { parser: require.resolve('babel-eslint') },
      ],
      [
        'class MyClass { async render(a, b) { return 3; } }',
        'class MyClass { render = async (a, b) => 3; }',
        { classPropertiesAllowed: true },
        { parser: require.resolve('babel-eslint') },
      ],
      [
        'class MyClass {async render(a, b) { return 3; } }',
        'class MyClass {render = async (a, b) => 3; }',
        { classPropertiesAllowed: true },
        { parser: require.resolve('babel-eslint') },
      ],
      ['var MyClass = { render(a, b) { return 3; }, b: false }', 'var MyClass = { render: (a, b) => 3, b: false }'],
      ['const foo = { barProp() { return "bar"; } };', 'const foo = { barProp: () => "bar" };'],
      // Make sure named function declarations work
      ['function foo() { return 3; }', 'const foo = () => 3;'],
      ['function foo(a) { return 3 }', 'const foo = (a) => 3;'],
      ['function foo(a) { return 3; }', 'const foo = (a) => 3;'],

      // Eslint treats export default as a special form of function declaration
      ['export default function() { return 3; }', 'export default () => 3;'],

      // Sanity check - make sure complex logic works
      ['function foo(a) { return a && (3 + a()) ? true : 99; }', 'const foo = (a) => a && (3 + a()) ? true : 99;'],

      // Make sure function expressions work
      ['var foo = function() { return "World"; }', 'var foo = () => "World"'],
      ['var foo = function() { return "World"; };', 'var foo = () => "World";'],
      ['var foo = function x() { return "World"; };', 'var foo = () => "World";'],

      // Make sure we wrap object literal returns in parens
      ['var foo = function() { return {a: false} }', 'var foo = () => ({a: false})'],
      ['var foo = function() { return {a: false}; }', 'var foo = () => ({a: false})'],
      ['function foo(a) { return {a: false}; }', 'const foo = (a) => ({a: false});'],
      ['function foo(a) { return {a: false} }', 'const foo = (a) => ({a: false});'],

      // Make sure we treat inner functions properly
      ['var foo = function () { return function(a) { a() } }', 'var foo = () => function(a) { a() }'],
      ['var foo = function () { return () => false }', 'var foo = () => () => false'],

      // Make sure we don't obliterate comments/whitespace and only remove newlines when appropriate
      ['var foo = function() {\n  return "World";\n}', 'var foo = () => "World"'],
      ['var foo = function() {\n  return "World"\n}', 'var foo = () => "World"'],
      ['function foo(a) {\n  return 3;\n}', 'const foo = (a) => 3;'],
      ['function foo(a) {\n  return 3\n}', 'const foo = (a) => 3;'],
      [
        '/*1*/var/*2*/ /*3*/foo/*4*/ /*5*/=/*6*/ /*7*/function/*8*/ /*9*/x/*10*/(/*11*/a/*12*/, /*13*/b/*14*/)/*15*/ /*16*/{/*17*/ /*18*/return/*19*/ /*20*/false/*21*/;/*22*/ /*23*/}/*24*/;/*25*/',
        '/*1*/var/*2*/ /*3*/foo/*4*/ /*5*/=/*6*/ /*7*//*8*/ /*9*//*10*/(/*11*/a/*12*/, /*13*/b/*14*/)/*15*/ /*16*/=> /*17*/ /*18*//*19*/ /*20*/false/*21*//*22*/ /*23*//*24*/;/*25*/',
      ],
      [
        '/*1*/function/*2*/ /*3*/foo/*4*/(/*5*/a/*6*/)/*7*/ /*8*/\{/*9*/ /*10*/return/*11*/ /*12*/false/*13*/;/*14*/ /*15*/}/*16*/',
        '/*1*/const/*2*/ /*3*/foo/*4*/ = (/*5*/a/*6*/)/*7*/ /*8*/=> /*9*/ /*10*//*11*/ /*12*/false/*13*//*14*/ /*15*/;/*16*/'
      ],

      // Make sure we don't mess up inner generator functions
      [
        'function foo() { return function * gen() { return yield 1; }; }',
        'const foo = () => function * gen() { return yield 1; };'
      ],

      // Make sure we don't mess with the semicolon in for statements 
      [
        'function withLoop() { return () => { for (i = 0; i < 5; i++) {}}}',
        'const withLoop = () => () => { for (i = 0; i < 5; i++) {}};'
      ],
      [
        'var withLoop = function() { return () => { for (i = 0; i < 5; i++) {}}}',
        'var withLoop = () => () => { for (i = 0; i < 5; i++) {}}'
      ],
      [
        'function withLoop() { return () => { for (i = 0; i < 5; i++) {}} /* foo */; }',
        'const withLoop = () => () => { for (i = 0; i < 5; i++) {}} /* foo */;'
      ],

      // Support async / await syntax
      ...[
        ['async function foo() { return "bar" }', 'const foo = async () => "bar";'],
        ['var foo = async function() { return "bar" };', 'var foo = async () => "bar";'],
        [
          'async function foo() { return await Promise.resolve("bar"); }',
          'const foo = async () => await Promise.resolve("bar");'
        ],
        [
          'var foo = async function() { return await Promise.resolve("bar") };',
          'var foo = async () => await Promise.resolve("bar");'
        ],
      ].map(asyncTest => [...asyncTest, null, { parserOptions: { ecmaVersion: 2017 } }]),

      // Support fixes with typescript typings
      ...[
        [
          'const foo = { bar(x: string) { return "bar"; } }',
          'const foo = { bar: (x: string) => "bar" }'
        ],
        [
          'const foo = { bar(x: string): string { return "bar"; } }',
          'const foo = { bar: (x: string): string => "bar" }'
        ],
        [
          'function foo(x: string): string { return x; }',
          'const foo = (x: string): string => x;'
        ],
        [
          'async function foo(x: number): Promise<number> { return x; }',
          'const foo = async (x: number): Promise<number> => x;'
        ],
        [
          'const nested = { foo: { bar(name: string) { return name; } } }',
          'const nested = { foo: { bar: (name: string) => name } }',
        ],
        [
          'const foo = function x(n: number): number { return n + 1; };',
          'const foo = (n: number): number => n + 1;',
        ],
        [
          'export function test(str: string): string { return str; }',
          'export const test = (str: string): string => str;',
        ],
        [
          'function str(n: number) { return n as string; }',
          'const str = (n: number) => n as string;',
        ]
      ].map(test => [...test, null, { parser: require.resolve('@typescript-eslint/parser') }])
    ].map(inputOutput => Object.assign(
      {
        errors: ['Prefer using arrow functions over plain functions which only return a value'],
        output: inputOutput[1],
        ...(inputOutput[3] || {}),
      },
      {
        ...singleReturnOnly(inputOutput[0], inputOutput[2]),
        parserOptions: {
          ...singleReturnOnly(inputOutput[0], inputOutput[2]).parserOptions,
          ...((inputOutput[3] || {}).parserOptions || {})
        },
      }
    )),

    {
      code: 'class MyClass { constructor() { this.x = 0; } add(y) { this.x += y; } }',
      errors: ['Prefer using arrow functions over plain functions'],
      options: [{ classPropertiesAllowed: true }],
      parser: require.resolve('babel-eslint')
    }
  ]
});
