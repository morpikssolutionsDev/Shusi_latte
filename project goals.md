# Goals

Sushi_latte is a horeca(hotel,restaurant,cafee) webapps service , this tools is used to multi point that has many modules 

## Crew Scoope

## Admin Panel
Acess Scoppe: Management

This module is for control the operational parameters, menu, price, discount etc

### HRIS
Acess Scoope: Management,Admin,Crew

A HRIS modules to manage employee, time off, attendace for all crews

### Waiter/Server

Acess Scoope: Service Crew/Waiter

This module is used as order note , replacing conventional paper base note to take order from customer, this module also can bind the order from client menu (customer)

Workflow:
Waiters can serve the customer directly from the table , taking the order note, or claim order by pushed notification, and the waiter confirm to the customer about the order

Waiter can also close order to the customer, show the order summary, and print recipt (no payement gateway), the printed out should be 2 , for the customer and for the internal as payment refrence


### Cashieer

A POS module for serving customer pay, this module should also have direct order from customer, similar to waiter/server module


### Kitchen
Mainly use as big display order queue , minimum input just for canceling, or processing orders used for kitchen crew, and can view SLA

## External Scoope

### Client Menu

A menu list for customers, customer can also make order from this module by scanning the web entry point, the web entry point is binned to the table number

Workflow:
Customer scan the barcode
Customer fill Full Name (required)
Customer fill additional medical fill, like lactose intolerant, etc (not required)
Customer order menus
After order complete, customer can trigger notification to waiters or can call waiter by verbal

customer can view the order SLA based on queue


### Product display showcase

Mainly use to showcase product list, menus, etc as banner

## Execution Order

1. Admin Panel
2. Client Menu
3. Product Display Showcase
4. Waiter/server module
5. Kitchen
6. Cashier
7. HRIS


## TechStack

Backend
Go

Front end
node.js
frammer motion
Shacdn UI
etc

