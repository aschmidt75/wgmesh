<template>
    <b-container id="main-container">
      <b-row>
        <b-badge></b-badge>
        <b-col class="col-6">
          <h3>Active mesh nodes
          <b-icon-chevron-double-down class="float" scale="0.5" v-b-toggle.collapse-mesh-info size="lg" ></b-icon-chevron-double-down>
          </h3>
          <b-collapse id="collapse-mesh-info" class="mt-8">
          <b-card>
            <p class="card-text">Mesh info</p>
          </b-card>
          </b-collapse>
          
         </b-col>
        <b-col class="col-6">
          <b-row>
            <b-col class="md">
            </b-col>
            <b-col class="md col-5">
                <b-form>
                  <b-form-input 
                    size="sm" 
                    class="mr" 
                    placeholder="Search in node data"
                    v-model="filter"
                    ></b-form-input>
                </b-form>
            </b-col>
            <b-col class="xs float-right col-2">
               <b-button-group>
                <b-button size="sm" variant="outline-dark" v-on:click="refresh()">
                  <b-icon-arrow-clockwise></b-icon-arrow-clockwise>
                </b-button>
              <b-button size="sm" variant="outline-dark" v-on:click="refresh()">
                <b-icon-bug-fill c1lass="float-right" v-if="withSystemTags" v-on:click="hideSystemTags" size="sm" ></b-icon-bug-fill>
                <b-icon-bug c1lass="float-right" v-else v-on:click="showSystemTags" size="sm" ></b-icon-bug>
              </b-button>
               </b-button-group>
            </b-col>
          </b-row>
        </b-col>
      </b-row>
        <div v-if="items !== undefined && items.length">
          <b-table small borderless outlined fixed
            :fields="fields" 
            :items="items"
            :sort-by.sync="sortBy"
            :sort-desc.sync="sortDesc" 
            :filter="filter"
            primary-key="name"
            head-variant="light"
            >
              <template #cell(name)="data">
                <span v-html="data.value"></span>
                <b v:if="{{ 1 == 2 }}">This node</b>
              </template>
              <template #cell(tags)="data">
                <span v-html="data.value"></span>
              </template>
          </b-table>
        </div>
        <div v-else>
          <h5>No active nodes in mesh</h5>
        </div>
    </b-container>
</template>

<script>
import axios from "axios";
import { from } from 'rxjs';
import { map, toArray } from 'rxjs/operators';
const util = require ('util')
import { BBadge, BIconArrowClockwise, BIconBug, BIconBugFill, BIconChevronDoubleDown } from 'bootstrap-vue'

export default {
  name: 'NodesTable',
  components: {
    BIconArrowClockwise,
    BIconBug,
    BIconBugFill,
    BIconChevronDoubleDown,
    BBadge,
  },
  data() {
    return {
          withSystemTags: false,
          sortBy: 'name',
          sortDesc: false,
          filter: null,
          filterOn: [],
          nodeNameOfSelf: "-none-",
          fields: [
            { 
              key: "name", 
              label: 'Node Name', 
              sortable: true, 
              stickyColumn: true ,
              filterByFormatted: true,
              formatter: (name) => {
                let str = name;
                if (name === this.nodeNameOfSelf) {
                  str += "<b-icon-bug></b-icon-bug>"
                }
                // find out if this is 
                return str
              }
            },
            { key: "meshIP", label: 'IP Address', sortable: true },
            { key: "rtt", label: 'RTT [msec]', sortable: true },
            { 
              key: "tags", 
              label: 'Tags', 
              sortable: false,
              filterByFormatted: true,
              formatter: (tag) => {
                let str = "<table>"
                for (let k in tag) {
                  if (this.withSystemTags == false && k.startsWith("_")) {
                    continue;
                  }
                  str += "<tr>"
                  const v = tag[k]
                  str +=  util.format("<td>%s</td><td>%s</td>", k, v);
                  str += "</tr>"
                }
                str += "</table>"
                return str
              }
            },
          ], 
          items: []
    };
  },
  mounted() {
    this.getFromAPI();
    this.getMock();
  },
  methods: {
    refresh() {
      this.getFromAPI();
    },
    hideSystemTags() {
      this.withSystemTags = false;
    },
    showSystemTags() {
      this.withSystemTags = true;
    },
    getMock() {
      this.nodeNameOfSelf = "m1";
      this.items = [
            { "name": "m1", "meshIP": "10.0.0.1", "rtt": 20, "tags": { "_srv": "none", "_ip": "1.2.3.4", "type": "app" } },
            { "name": "m2", "meshIP": "10.0.12.34", isSelf: true,"rtt": 128, "tags": { "_srv": "none", "type": "nginx" } }
      ]
    },
    getFromAPI() {
      axios
        .get("/api/nodes")
        .then(response => {
          if (response !== undefined && response.data !== undefined && response.data.nodes !== undefined) {
            const mappedData = from(response.data.nodes).pipe(
              map(elem => {
                return {
                  "name": elem.name,
                  "meshIP": elem.meshIP,
                  "tags": elem.tags
                }
              }),
              toArray()
            );
            mappedData.subscribe(res => {
              this.items = res
            })

          }
        })        
        .catch(err => {
          console.log(err);
        });
    }
  }
};
</script>


<style scoped>
#main-container > div {
  padding-top: 1rem;
}
</style>
