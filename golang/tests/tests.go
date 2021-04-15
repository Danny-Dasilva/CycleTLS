// // Futures API -- async await
// func Fetch(name string) <-chan Item {
// 	c := make(chan Item, 1)
// 	go func () {
// 		[...]
// 		c <-item
// 	}()
// 	return  c
// }

// a := Fetch("a")
// b := Fetch("b")
// Consume(<-a, <-b)


// //Producer consumer queue
// func Glob(pattern string) <-chan Item {
// 	c := make(chan Item)
// 	go func () {
// 		defer close(c)
// 		for {
// 			[...]
// 			c <-item
// 		}
		
// 	}()
// 	return  c
// }

// //multiplex accross OS threads
// //above avoids blocking
// // reduce idle threads
// //the two above don't apply in go
